package api

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ondrejsalek/cjp/internal/billing"
)

const webhookPath = "/api/billing/webhook"

// BillingConfig enables paid API keys (higher rate limits). The free API is unchanged.
// Billing is off unless Keys and Stripe are set.
type BillingConfig struct {
	Keys       *billing.Keys
	Stripe     *billing.Stripe
	PriceID    string
	PublicURL  string // e.g. https://cjp.example.com — return URLs are never built from the Host header
	PriceLabel string // shown on the website, e.g. "9 € měsíčně"
	RateLimit  RateLimitConfig
}

func (c BillingConfig) enabled() bool { return c.Keys != nil && c.Stripe != nil }

func (s *Server) billingRoutes() {
	if !s.billing.enabled() {
		return
	}
	s.mux.HandleFunc("POST /api/billing/checkout", s.handleCheckout)
	s.mux.HandleFunc("GET /api/billing/key", s.handleKeyPage)
	s.mux.HandleFunc("POST /api/billing/key", s.handleKeyIssue)
	s.mux.HandleFunc("POST /api/billing/portal", s.handlePortal)
	s.mux.HandleFunc("POST "+webhookPath, s.handleStripeWebhook)
}

// bearerKey returns the API key from "Authorization: Bearer cjp_…", or "".
func bearerKey(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	u, err := s.billing.Stripe.CheckoutURL(r.Context(), s.billing.PriceID, s.billing.PublicURL+"/api/billing/key", s.billing.PublicURL+"/")
	if err != nil {
		log.Printf("billing: checkout: %v", err)
		writeError(w, http.StatusBadGateway, "Platební bránu se nepodařilo otevřít, zkuste to prosím později.")
		return
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var keyPage = template.Must(template.New("key").Parse(`<!doctype html>
<html lang="cs"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><title>API klíč · CJP</title>
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:40rem;margin:3rem auto;padding:0 1rem;color:#0f172a}
code,pre{background:#f1f5f9;border-radius:.4rem;padding:.15rem .4rem;word-break:break-all}pre{padding:.8rem;white-space:pre-wrap}
button{font:inherit;padding:.5rem .9rem;border-radius:.5rem;border:1px solid #1d4ed8;background:#1d4ed8;color:#fff;cursor:pointer}
button.secondary{background:#fff;color:#1d4ed8}form{display:inline-block;margin:.25rem .5rem .25rem 0}.warn{color:#b91c1c}</style></head><body>
<h1>API klíč CJP</h1>
{{if .Error}}<p class="warn" role="alert">{{.Error}}</p>{{end}}
{{if .Key}}
<p>Váš klíč (zobrazí se jen teď, uložte si ho):</p>
<pre>{{.Key}}</pre>
<p>Použití:</p>
<pre>curl -H "Authorization: Bearer {{.Key}}" "{{.PublicURL}}/api/v1/words/žena"</pre>
{{else if .Paying}}
<p>{{if .HasKey}}Klíč k tomuto předplatnému už byl vydán. Vygenerováním nového přestane starý platit.{{else}}Platba proběhla. Vygenerujte si klíč:{{end}}</p>
<form method="post" action="/api/billing/key"><input type="hidden" name="session_id" value="{{.SessionID}}">
<button type="submit">{{if .HasKey}}Vygenerovat nový klíč{{else}}Vygenerovat klíč{{end}}</button></form>
{{end}}
{{if .SessionID}}
<form method="post" action="/api/billing/portal"><input type="hidden" name="session_id" value="{{.SessionID}}">
<button type="submit" class="secondary">Správa předplatného a faktury</button></form>
<p><small>Tuto stránku si uložte do záložek: přes ni se dostanete ke správě předplatného a k vydání nového klíče.</small></p>
{{end}}
<p><a href="/">← Zpět na slovník</a></p>
</body></html>`))

type keyPageData struct {
	SessionID, Key, Error, PublicURL string
	Paying, HasKey                   bool
}

func (s *Server) renderKeyPage(w http.ResponseWriter, status int, d keyPageData) {
	d.PublicURL = s.billing.PublicURL
	// The page shows a secret and its URL carries the session id.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = keyPage.Execute(w, d)
}

// payingSession re-checks with Stripe on every call: a finished checkout of this product whose
// subscription is still paid. Writes the error page itself and returns nil when it is not.
func (s *Server) payingSession(w http.ResponseWriter, r *http.Request, sessionID string) *billing.Session {
	sess, err := s.billing.Stripe.GetSession(r.Context(), sessionID)
	if err != nil || !sess.Ours {
		s.renderKeyPage(w, http.StatusNotFound, keyPageData{Error: "Tuto platbu se nepodařilo najít."})
		return nil
	}
	if !sess.Paying {
		s.renderKeyPage(w, http.StatusPaymentRequired, keyPageData{SessionID: sessionID, Error: "Předplatné není aktivní. Obnovit ho můžete ve správě předplatného."})
		return nil
	}
	return sess
}

func (s *Server) handleKeyPage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	sess := s.payingSession(w, r, sessionID)
	if sess == nil {
		return
	}
	hasKey, err := s.billing.Keys.Exists(r.Context(), sess.SubscriptionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.renderKeyPage(w, http.StatusOK, keyPageData{SessionID: sessionID, Paying: true, HasKey: hasKey})
}

// handleKeyIssue creates or rotates the key. POST only, so opening or prefetching the return
// link never changes a key.
func (s *Server) handleKeyIssue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.FormValue("session_id")
	sess := s.payingSession(w, r, sessionID)
	if sess == nil {
		return
	}
	key, err := s.billing.Keys.Issue(r.Context(), sess.SubscriptionID)
	if err != nil {
		log.Printf("billing: issue key: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.renderKeyPage(w, http.StatusOK, keyPageData{SessionID: sessionID, Key: key})
}

func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
	sessionID := r.FormValue("session_id")
	sess, err := s.billing.Stripe.GetSession(r.Context(), sessionID)
	if err != nil || !sess.Ours || sess.CustomerID == "" {
		s.renderKeyPage(w, http.StatusNotFound, keyPageData{Error: "Tuto platbu se nepodařilo najít."})
		return
	}
	u, err := s.billing.Stripe.PortalURL(r.Context(), sess.CustomerID, s.billing.PublicURL+"/api/billing/key?session_id="+sessionID)
	if err != nil {
		log.Printf("billing: portal: %v", err)
		writeError(w, http.StatusBadGateway, "Správu předplatného se nepodařilo otevřít, zkuste to prosím později.")
		return
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
		return
	}
	ev, err := s.billing.Stripe.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad signature")
		return
	}
	// Setting a flag from the event's status is idempotent, so replays need no bookkeeping.
	// shortcut: trusts delivery order; re-fetch the subscription from Stripe if stale events ever flip a key.
	if ev != nil {
		if err := s.billing.Keys.SetActive(r.Context(), ev.SubscriptionID, ev.Active); err != nil {
			log.Printf("billing: webhook: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error") // Stripe retries
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
