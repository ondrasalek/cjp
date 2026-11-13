package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Product is written into Stripe metadata so sessions of other products on the same
// Stripe account can never mint keys here.
const Product = "cjp"

// Stripe is the handful of REST calls this service needs; the official SDK would be a large
// dependency for three requests and one HMAC.
type Stripe struct {
	SecretKey     string
	WebhookSecret string
	BaseURL       string // default https://api.stripe.com; overridden in tests
	HTTP          *http.Client
}

func (s *Stripe) call(ctx context.Context, method, path string, form url.Values, out any) error {
	base := s.BaseURL
	if base == "" {
		base = "https://api.stripe.com"
	}
	var body io.Reader
	target := base + path
	if method == http.MethodGet {
		if len(form) > 0 {
			target += "?" + form.Encode()
		}
	} else {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.SecretKey, "")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("stripe %s %s: HTTP %d", method, path, res.StatusCode)
	}
	return json.Unmarshal(raw, out)
}

// CheckoutURL starts a subscription checkout and returns the hosted page's URL.
// returnURL receives ?session_id=<id> after payment.
func (s *Stripe) CheckoutURL(ctx context.Context, priceID, returnURL, cancelURL string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := s.call(ctx, http.MethodPost, "/v1/checkout/sessions", url.Values{
		"mode":                                 {"subscription"},
		"line_items[0][price]":                 {priceID},
		"line_items[0][quantity]":              {"1"},
		"success_url":                          {returnURL + "?session_id={CHECKOUT_SESSION_ID}"},
		"cancel_url":                           {cancelURL},
		"allow_promotion_codes":                {"true"},
		"metadata[product]":                    {Product},
		"subscription_data[metadata][product]": {Product},
	}, &out)
	if err == nil && out.URL == "" {
		err = errors.New("stripe returned no checkout url")
	}
	return out.URL, err
}

// Session is what we need to know about a finished checkout.
type Session struct {
	SubscriptionID string
	CustomerID     string
	Paying         bool // completed checkout of this product with a subscription that is active or trialing
	Ours           bool // belongs to this product at all
}

// GetSession fetches a checkout session with its subscription.
func (s *Stripe) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// The id goes into the path of an authenticated Stripe request, so only the known shape passes.
	if !sessionIDPattern.MatchString(sessionID) {
		return nil, errors.New("not a checkout session id")
	}
	var out struct {
		Status   string            `json:"status"`
		Customer string            `json:"customer"`
		Metadata map[string]string `json:"metadata"`
		Sub      *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"subscription"`
	}
	if err := s.call(ctx, http.MethodGet, "/v1/checkout/sessions/"+sessionID, url.Values{"expand[]": {"subscription"}}, &out); err != nil {
		return nil, err
	}
	sess := &Session{CustomerID: out.Customer, Ours: out.Metadata["product"] == Product}
	if out.Sub != nil {
		sess.SubscriptionID = out.Sub.ID
		sess.Paying = sess.Ours && out.Status == "complete" && paying(out.Sub.Status)
	}
	return sess, nil
}

func paying(status string) bool { return status == "active" || status == "trialing" }

// PortalURL opens Stripe's customer portal (cancel, card, invoices).
func (s *Stripe) PortalURL(ctx context.Context, customerID, returnURL string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := s.call(ctx, http.MethodPost, "/v1/billing_portal/sessions", url.Values{"customer": {customerID}, "return_url": {returnURL}}, &out)
	if err == nil && out.URL == "" {
		err = errors.New("stripe returned no portal url")
	}
	return out.URL, err
}

// SubscriptionEvent is the part of a customer.subscription.* webhook we act on.
type SubscriptionEvent struct {
	SubscriptionID string
	Active         bool
}

var sessionIDPattern = regexp.MustCompile(`^cs_[A-Za-z0-9_]{1,200}$`)

const webhookTolerance = 5 * time.Minute

// VerifyWebhook checks the Stripe-Signature header (HMAC-SHA256 over "<t>.<payload>") and returns
// the subscription change, or nil for event types we ignore.
func (s *Stripe) VerifyWebhook(payload []byte, header string, now time.Time) (*SubscriptionEvent, error) {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return nil, errors.New("malformed signature header")
	}
	if d := now.Sub(time.Unix(sec, 0)); d > webhookTolerance || d < -webhookTolerance {
		return nil, errors.New("signature timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(s.WebhookSecret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	want := mac.Sum(nil)
	valid := false
	for _, sig := range sigs {
		got, err := hex.DecodeString(sig)
		if err == nil && hmac.Equal(got, want) {
			valid = true
		}
	}
	if !valid {
		return nil, errors.New("signature mismatch")
	}

	var ev struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, err
	}
	switch ev.Type {
	case "customer.subscription.updated":
		return &SubscriptionEvent{SubscriptionID: ev.Data.Object.ID, Active: paying(ev.Data.Object.Status)}, nil
	case "customer.subscription.deleted":
		return &SubscriptionEvent{SubscriptionID: ev.Data.Object.ID, Active: false}, nil
	}
	return nil, nil
}
