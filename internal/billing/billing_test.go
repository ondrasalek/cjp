package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Payload and header produced by the official Stripe Node SDK
// (stripe.webhooks.generateTestHeaderString with secret "whsec_fixture", timestamp 1800000000),
// so this checks our verifier against Stripe's implementation, not against our own signer.
const (
	fixturePayload = `{"id":"evt_1","type":"customer.subscription.deleted","data":{"object":{"id":"sub_1","status":"canceled"}}}`
	fixtureHeader  = "t=1800000000,v1=7401ccda61b5fca50c0769913379529474424291d353cfead903648f9d2d9d94"
)

func TestVerifyWebhook(t *testing.T) {
	s := &Stripe{WebhookSecret: "whsec_fixture"}
	at := time.Unix(1800000000, 0)

	ev, err := s.VerifyWebhook([]byte(fixturePayload), fixtureHeader, at.Add(time.Minute))
	if err != nil || ev == nil || ev.SubscriptionID != "sub_1" || ev.Active {
		t.Fatalf("valid fixture: ev=%+v err=%v", ev, err)
	}
	bad := map[string]struct {
		payload, header string
		now             time.Time
	}{
		"tampered payload": {strings.Replace(fixturePayload, "sub_1", "sub_2", 1), fixtureHeader, at},
		"wrong signature":  {fixturePayload, "t=1800000000,v1=" + strings.Repeat("0", 64), at},
		"no signature":     {fixturePayload, "", at},
		"replayed late":    {fixturePayload, fixtureHeader, at.Add(time.Hour)},
	}
	for name, c := range bad {
		if _, err := s.VerifyWebhook([]byte(c.payload), c.header, c.now); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := (&Stripe{WebhookSecret: "whsec_other"}).VerifyWebhook([]byte(fixturePayload), fixtureHeader, at); err == nil {
		t.Error("wrong secret: expected an error")
	}
}

func TestStripeCalls(t *testing.T) {
	var gotForm map[string]string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, _, _ := r.BasicAuth(); user != "sk_test_x" {
			t.Errorf("missing secret key auth on %s", r.URL.Path)
		}
		_ = r.ParseForm()
		gotForm = map[string]string{}
		for k := range r.Form {
			gotForm[k] = r.Form.Get(k)
		}
		switch r.URL.Path {
		case "/v1/checkout/sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{"url": "https://checkout.test/pay"})
		case "/v1/checkout/sessions/cs_ok":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "complete", "customer": "cus_1", "metadata": map[string]string{"product": "cjp"},
				"subscription": map[string]any{"id": "sub_1", "status": "active"},
			})
		case "/v1/checkout/sessions/cs_other":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "complete", "customer": "cus_2", "metadata": map[string]string{"product": "something-else"},
				"subscription": map[string]any{"id": "sub_2", "status": "active"},
			})
		default:
			http.Error(w, `{"error":{}}`, http.StatusNotFound)
		}
	}))
	defer fake.Close()
	s := &Stripe{SecretKey: "sk_test_x", BaseURL: fake.URL}
	ctx := context.Background()

	u, err := s.CheckoutURL(ctx, "price_1", "https://cjp.test/api/billing/key", "https://cjp.test/")
	if err != nil || u != "https://checkout.test/pay" {
		t.Fatalf("CheckoutURL: %q %v", u, err)
	}
	for k, want := range map[string]string{
		"mode": "subscription", "line_items[0][price]": "price_1", "metadata[product]": "cjp",
		"subscription_data[metadata][product]": "cjp",
		"success_url":                          "https://cjp.test/api/billing/key?session_id={CHECKOUT_SESSION_ID}",
	} {
		if gotForm[k] != want {
			t.Errorf("checkout form %s = %q, want %q", k, gotForm[k], want)
		}
	}

	ok, err := s.GetSession(ctx, "cs_ok")
	if err != nil || !ok.Paying || !ok.Ours || ok.SubscriptionID != "sub_1" || ok.CustomerID != "cus_1" {
		t.Fatalf("GetSession ok: %+v %v", ok, err)
	}
	if gotForm["expand[]"] != "subscription" {
		t.Errorf("session fetch must expand the subscription, got %v", gotForm)
	}
	other, err := s.GetSession(ctx, "cs_other")
	if err != nil || other.Ours || other.Paying {
		t.Fatalf("another product's session must not count: %+v %v", other, err)
	}
	for _, id := range []string{"", "sub_1", "cs_x/../../v1/customers", "cs_ok?expand[]=customer", "cs_ok%2F..", "cs_missing"} {
		if _, err := s.GetSession(ctx, id); err == nil {
			t.Errorf("GetSession(%q): expected an error", id)
		}
	}
}

func TestKeys(t *testing.T) {
	keys, err := OpenKeys(filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer keys.Close()
	ctx := context.Background()

	first, err := keys.Issue(ctx, "sub_1")
	if err != nil || !strings.HasPrefix(first, KeyPrefix) {
		t.Fatalf("Issue: %q %v", first, err)
	}
	if _, ok := keys.Lookup(ctx, first); !ok {
		t.Fatal("fresh key must be valid")
	}
	if _, ok := keys.Lookup(ctx, "cjp_nope"); ok {
		t.Fatal("unknown key must be invalid")
	}
	second, _ := keys.Issue(ctx, "sub_1") // rotation
	if _, err := keys.Issue(ctx, "sub_2"); err != nil {
		t.Fatal(err)
	}
	_ = keys.SetActive(ctx, "sub_2", false)
	if s, err := keys.Stats(ctx); err != nil || s != (Stats{Total: 2, Active: 1, Last7: 2, Last30: 2}) {
		t.Fatalf("Stats: %+v %v", s, err)
	}
	if _, ok := keys.Lookup(ctx, first); ok {
		t.Fatal("rotated-out key must stop working")
	}
	if err := keys.SetActive(ctx, "sub_1", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys.Lookup(ctx, second); ok {
		t.Fatal("key of a lapsed subscription must stop working")
	}
	if has, _ := keys.Exists(ctx, "sub_1"); !has {
		t.Fatal("Exists must see the issued key")
	}
	if has, _ := keys.Exists(ctx, "sub_none"); has {
		t.Fatal("Exists must be false for unknown subscriptions")
	}
}
