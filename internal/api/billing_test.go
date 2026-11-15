package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ondrejsalek/cjp/internal/billing"
)

// newBillingServer wires the API to a fake Stripe. Webhook signature checking itself is covered
// in internal/billing against an SDK-generated fixture.
func newBillingServer(t *testing.T) (http.Handler, *billing.Keys, *url.Values) {
	t.Helper()
	lastCheckout := &url.Values{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := func(product, subStatus string) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "complete", "customer": "cus_1", "metadata": map[string]string{"product": product},
				"subscription": map[string]any{"id": "sub_1", "status": subStatus},
			})
		}
		switch r.URL.Path {
		case "/v1/checkout/sessions":
			_ = r.ParseForm()
			*lastCheckout = r.Form
			_ = json.NewEncoder(w).Encode(map[string]any{"url": "https://checkout.test/pay"})
		case "/v1/checkout/sessions/cs_ok":
			session("cjp", "active")
		case "/v1/checkout/sessions/cs_lapsed":
			session("cjp", "canceled")
		case "/v1/checkout/sessions/cs_other":
			session("nodwait", "active")
		case "/v1/billing_portal/sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{"url": "https://portal.test/x"})
		default:
			http.Error(w, "{}", http.StatusNotFound)
		}
	}))
	t.Cleanup(fake.Close)
	keys, err := billing.OpenKeys(filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keys.Close() })
	srv := NewServer(Config{
		RateLimit: RateLimitConfig{RPS: 0.001, Burst: 3},
		Billing: BillingConfig{
			Keys: keys, Stripe: &billing.Stripe{SecretKey: "sk_test_x", WebhookSecret: "whsec_t", BaseURL: fake.URL},
			PriceID: "price_1", PublicURL: "https://cjp.test", RateLimit: RateLimitConfig{RPS: 0.001, Burst: 5},
			AdminPassword: testAdminPassword,
		},
	})
	return srv.Handler(), keys, lastCheckout
}

func do(h http.Handler, method, target, ip, key string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader = strings.NewReader(form.Encode())
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Host = "attacker.example"
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const testAdminPassword = "test-admin-password-1"

var keyRe = regexp.MustCompile(`cjp_[A-Za-z0-9_-]{20,}`)

func TestPaidKeyFlow(t *testing.T) {
	h, keys, lastCheckout := newBillingServer(t)
	// An unknown /api/ path reaches the mux (404) only after passing the limiter and key check.
	const probe = "/api/v1/probe"

	rec := do(h, "POST", "/api/billing/checkout", "203.0.113.1", "", url.Values{})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://checkout.test/pay" {
		t.Fatalf("checkout: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if got := lastCheckout.Get("success_url"); !strings.HasPrefix(got, "https://cjp.test/") {
		t.Fatalf("return URL must come from the configured public URL, not the Host header: %q", got)
	}

	// Opening the return link shows the page but must not mint a key.
	rec = do(h, "GET", "/api/billing/key?session_id=cs_ok", "203.0.113.1", "", nil)
	if rec.Code != http.StatusOK || keyRe.MatchString(rec.Body.String()) {
		t.Fatalf("GET key page: %d, leaked key=%v", rec.Code, keyRe.MatchString(rec.Body.String()))
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("key page must be no-store and no-referrer: %v", rec.Header())
	}
	if has, _ := keys.Exists(t.Context(), "sub_1"); has {
		t.Fatal("GET must not create a key")
	}

	// Billing endpoints share the per-IP free limit, so each case comes from its own address.
	for i, c := range []struct {
		session string
		want    int
	}{{"cs_other", http.StatusNotFound}, {"cs_lapsed", http.StatusPaymentRequired}, {"cs_missing", http.StatusNotFound}, {"", http.StatusNotFound}} {
		rec = do(h, "POST", "/api/billing/key", fmt.Sprintf("203.0.113.%d", 10+i), "", url.Values{"session_id": {c.session}})
		if rec.Code != c.want || keyRe.MatchString(rec.Body.String()) {
			t.Errorf("issue with %q: got %d (key shown: %v), want %d", c.session, rec.Code, keyRe.MatchString(rec.Body.String()), c.want)
		}
	}

	rec = do(h, "POST", "/api/billing/key", "203.0.113.20", "", url.Values{"session_id": {"cs_ok"}})
	first := keyRe.FindString(rec.Body.String())
	if rec.Code != http.StatusOK || first == "" {
		t.Fatalf("issue: %d, no key in page", rec.Code)
	}

	// Paid key: its own, larger bucket (burst 5), independent of the caller's IP bucket (burst 3).
	for i := 1; i <= 5; i++ {
		rec = do(h, "GET", probe, "198.51.100.7", first, nil)
		if rec.Code != http.StatusNotFound || rec.Header().Get("X-RateLimit-Limit") != "5" {
			t.Fatalf("paid request %d: %d limit=%s", i, rec.Code, rec.Header().Get("X-RateLimit-Limit"))
		}
	}
	if rec = do(h, "GET", probe, "198.51.100.8", first, nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th paid request must hit the per-key limit even from another IP, got %d", rec.Code)
	}
	if rec = do(h, "GET", probe, "198.51.100.7", "", nil); rec.Code != http.StatusNotFound || rec.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("anonymous request keeps the free limit: %d limit=%s", rec.Code, rec.Header().Get("X-RateLimit-Limit"))
	}

	// Wrong keys get 401 and burn the IP's free budget.
	for i := 1; i <= 3; i++ {
		if rec = do(h, "GET", probe, "192.0.2.50", "cjp_wrong", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong key %d: got %d", i, rec.Code)
		}
	}
	if rec = do(h, "GET", probe, "192.0.2.50", "cjp_wrong", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("key guessing must be throttled, got %d", rec.Code)
	}

	// Rotation invalidates the old key.
	rec = do(h, "POST", "/api/billing/key", "203.0.113.2", "", url.Values{"session_id": {"cs_ok"}})
	second := keyRe.FindString(rec.Body.String())
	if second == "" || second == first {
		t.Fatal("rotation must show a new key")
	}
	if rec = do(h, "GET", probe, "203.0.113.3", first, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old key after rotation: %d", rec.Code)
	}

	rec = do(h, "POST", "/api/billing/portal", "203.0.113.21", "", url.Values{"session_id": {"cs_lapsed"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://portal.test/x" {
		t.Fatalf("portal must work for a lapsed subscription too: %d", rec.Code)
	}
	if rec = do(h, "POST", "/api/billing/portal", "203.0.113.22", "", url.Values{"session_id": {"cs_other"}}); rec.Code != http.StatusNotFound {
		t.Fatalf("portal for another product's session: %d", rec.Code)
	}

	// Webhook: unsigned is refused and not rate limited; the key store is what it flips.
	for i := 0; i < 6; i++ {
		rec = do(h, "POST", webhookPath, "192.0.2.99", "", url.Values{"x": {"y"}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unsigned webhook %d: got %d, want 400 (and never 429)", i, rec.Code)
		}
	}
	if _, ok := keys.Lookup(t.Context(), second); !ok {
		t.Fatal("an unsigned webhook must not change anything")
	}
	// A correctly signed cancellation, posted to the route, turns the key off.
	payload := `{"type":"customer.subscription.deleted","data":{"object":{"id":"sub_1","status":"canceled"}}}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("whsec_t"))
	mac.Write([]byte(ts + "." + payload))
	req := httptest.NewRequest("POST", webhookPath, strings.NewReader(payload))
	req.Header.Set("Stripe-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	req.RemoteAddr = "192.0.2.99:1"
	hook := httptest.NewRecorder()
	h.ServeHTTP(hook, req)
	if hook.Code != http.StatusOK {
		t.Fatalf("signed webhook: got %d", hook.Code)
	}
	if rec = do(h, "GET", probe, "203.0.113.4", second, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("key of a cancelled subscription: %d", rec.Code)
	}
}

func TestAdminPageNeedsThePassword(t *testing.T) {
	h, keys, _ := newBillingServer(t)
	if _, err := keys.Issue(t.Context(), "sub_admin"); err != nil {
		t.Fatal(err)
	}
	get := func(ip, pass string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/billing/admin", nil)
		req.RemoteAddr = ip + ":1234"
		if pass != "" {
			req.SetBasicAuth("anyone", pass)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for _, pass := range []string{"", "wrong", testAdminPassword + "x"} {
		rec := get("203.0.113.9", pass)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") == "" || strings.Contains(rec.Body.String(), "Klíčů celkem") {
			t.Fatalf("password %q: status %d, body %q", pass, rec.Code, rec.Body.String())
		}
	}
	// Guessing shares the per-IP limit of every other route: the fourth try from that address is refused unseen.
	if rec := get("203.0.113.9", testAdminPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after three wrong guesses: status %d", rec.Code)
	}
	rec := get("203.0.113.10", testAdminPassword)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<dt>Klíčů celkem</dt><dd>1</dd>") {
		t.Fatalf("admin page: status %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestBillingOffByDefault(t *testing.T) {
	h := NewServer(Config{RateLimit: RateLimitConfig{RPS: 1, Burst: 3}}).Handler()
	if rec := do(h, "POST", "/api/billing/checkout", "203.0.113.1", "", url.Values{}); rec.Code == http.StatusSeeOther {
		t.Fatal("checkout must not exist when billing is not configured")
	}
	// A stray Authorization header is ignored: the free API keeps working.
	if rec := do(h, "GET", "/api/v1/probe", "203.0.113.1", "cjp_whatever", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("free API with a stray key header: %d", rec.Code)
	}
}

func TestClientIPTrustsOnlyTheInternalProxy(t *testing.T) {
	ip := func(peer string, headers map[string]string) string {
		r := httptest.NewRequest("GET", "/api/v1/probe", nil)
		r.RemoteAddr = peer
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return extractClientIP(r)
	}
	// nginx (private peer) tells us the visitor in X-Real-IP.
	if got := ip("172.18.0.5:4000", map[string]string{"X-Real-IP": "203.0.113.9"}); got != "203.0.113.9" {
		t.Errorf("X-Real-IP from the internal proxy: got %s", got)
	}
	// Headers a client can set are never believed, even via the proxy.
	forged := map[string]string{"X-Real-IP": "203.0.113.9", "CF-Connecting-IP": "1.1.1.1", "X-Forwarded-For": "2.2.2.2, 3.3.3.3"}
	if got := ip("172.18.0.5:4000", forged); got != "203.0.113.9" {
		t.Errorf("forged CF/XFF headers must not win over X-Real-IP: got %s", got)
	}
	if got := ip("172.18.0.5:4000", map[string]string{"CF-Connecting-IP": "1.1.1.1", "X-Forwarded-For": "2.2.2.2"}); got != "172.18.0.5" {
		t.Errorf("without X-Real-IP the peer is the client: got %s", got)
	}
	// A public peer talks to the API directly: nothing it sends is trusted.
	if got := ip("198.51.100.7:4000", forged); got != "198.51.100.7" {
		t.Errorf("public peer: got %s", got)
	}
}
