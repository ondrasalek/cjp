package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondrejsalek/cjp/internal/db"
	"github.com/ondrejsalek/cjp/internal/hunspell"
	"github.com/ondrejsalek/cjp/internal/hyphen"
)

func setupRealTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join("..", "..", "data", "cjp.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("skipping integration test: %s not built yet", dbPath)
	}

	store, err := db.Open(dbPath, true)
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	hyph, err := hyphen.LoadFile(filepath.Join("..", "..", "data", "raw", "hyph_cs_CZ.dic"))
	if err != nil {
		t.Fatalf("LoadFile hyph: %v", err)
	}

	hun, err := hunspell.Load(
		filepath.Join("..", "..", "data", "raw", "cs_CZ.aff"),
		filepath.Join("..", "..", "data", "raw", "cs_CZ.dic"),
	)
	if err != nil {
		t.Fatalf("Load hunspell: %v", err)
	}

	return NewServer(Config{
		Store:      store,
		Hyphenator: hyph,
		Hunspell:   hun,
	})
}

func TestIntegrationEndpoints(t *testing.T) {
	srv := setupRealTestServer(t)
	h := srv.Handler()

	getJSON := func(path string, wantStatus int) map[string]any {
		t.Helper()
		t0 := time.Now()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		elapsed := time.Since(t0)

		if rec.Code != wantStatus {
			t.Fatalf("GET %s -> status %d (want %d): %s", path, rec.Code, wantStatus, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("GET %s invalid JSON: %v", path, err)
		}
		t.Logf("GET %-48s -> %d in %s", path, rec.Code, elapsed.Round(time.Microsecond))
		return out
	}

	// 1. Meta stats
	meta := getJSON("/api/v1/meta", http.StatusOK)
	stats := meta["stats"].(map[string]any)
	if stats["lexemes"].(float64) < 250000 {
		t.Errorf("expected >250k lexemes, got %v", stats["lexemes"])
	}
	t.Logf("DB Stats: lexemes=%.0f, word_forms=%.0f, senses=%.0f, relations=%.0f, translations=%.0f",
		stats["lexemes"], stats["word_forms"], stats["senses"], stats["relations"], stats["translations"])

	// 2. Complete word card for "žena" with EN & RU translations
	zena := getJSON("/api/v1/words/žena?langs=en,ru", http.StatusOK)
	if zena["exact_lemma_match"] != true {
		t.Errorf("expected exact_lemma_match=true for žena")
	}
	entries := zena["entries"].([]any)
	if len(entries) == 0 {
		t.Fatalf("expected entries for žena")
	}

	// 3. Inflected lookup "/api/v1/words/ženami" -> resolves to lemma "žena" with case 7 plural
	zenami := getJSON("/api/v1/words/ženami", http.StatusOK)
	if zenami["exact_lemma_match"] != false {
		t.Errorf("expected exact_lemma_match=false for ženami")
	}
	fms := zenami["form_matches"].([]any)
	if len(fms) == 0 {
		t.Errorf("expected form_matches for ženami")
	}

	// 4. ASCII fallback "/api/v1/words/zena" -> resolves to "žena"
	zenaASCII := getJSON("/api/v1/words/zena", http.StatusOK)
	if zenaASCII["ascii_fallback"] != true {
		t.Errorf("expected ascii_fallback=true for zena")
	}

	// 5. Verb conjugation "/api/v1/words/dělat"
	delat := getJSON("/api/v1/words/dělat?langs=en,ru", http.StatusOK)
	if len(delat["entries"].([]any)) == 0 {
		t.Errorf("expected entries for dělat")
	}

	// 6. Morphological analysis & validation
	validRes := getJSON("/api/v1/analyze?q=Aakjaerovými", http.StatusOK)
	if validRes["valid"] != true {
		t.Errorf("expected Aakjaerovými to be valid")
	}
	invalidRes := getJSON("/api/v1/analyze?q=xyzneexistujicislovo", http.StatusOK)
	if invalidRes["valid"] != false {
		t.Errorf("expected nonsense word to be invalid")
	}

	// 7. Diacritic-insensitive autocomplete
	ac := getJSON("/api/v1/autocomplete?q=zen&limit=5", http.StatusOK)
	if ac["count"].(float64) < 3 {
		t.Errorf("expected >=3 suggestions for 'zen', got %v", ac["count"])
	}

	// 8. Hyphenation of complex Czech word
	hy := getJSON("/api/v1/hyphenate?q=nejneobhospodařovávatelnější", http.StatusOK)
	if hy["syllable_count"].(float64) < 8 {
		t.Errorf("expected >=8 syllables, got %v (%v)", hy["syllable_count"], hy["hyphenated"])
	}

	// 9. Dedicated translations endpoint
	tr := getJSON("/api/v1/translations/pes?langs=en,ru", http.StatusOK)
	if len(tr["results"].([]any)) == 0 {
		t.Errorf("expected EN/RU translations for pes")
	}

	// 10. Full-text definition search without diacritics
	srch := getJSON("/api/v1/search?q=psovita+selma", http.StatusOK)
	if srch["count"].(float64) < 1 {
		t.Errorf("expected FTS hit for 'psovita selma'")
	}

	// 11. Fallback translations from base word for comparative adverb "horizontálněji"
	horiz := getJSON("/api/v1/words/horizontálněji?langs=en,ru", http.StatusOK)
	horizEntries := horiz["entries"].([]any)
	if len(horizEntries) == 0 {
		t.Fatalf("expected entries for horizontálněji")
	}
	firstHoriz := horizEntries[0].(map[string]any)
	if firstHoriz["translations_from_lemma"] != "horizontální" {
		t.Errorf("expected translations_from_lemma='horizontální', got %v", firstHoriz["translations_from_lemma"])
	}
	if len(firstHoriz["translations"].([]any)) == 0 {
		t.Errorf("expected fallback translations on horizontálnější")
	}

	// 12. Dedicated translations endpoint also resolves derived/inflected forms
	horizTr := getJSON("/api/v1/translations/horizontálněji?langs=en,ru", http.StatusOK)
	horizRes := horizTr["results"].([]any)
	if len(horizRes) == 0 {
		t.Fatalf("expected translation results for horizontálněji")
	}
	if horizRes[0].(map[string]any)["translations_from_lemma"] != "horizontální" {
		t.Errorf("expected translations_from_lemma='horizontální' on /translations/horizontálněji")
	}
}

func TestRateLimitAndQueryCaps(t *testing.T) {
	srv := setupRealTestServer(t)
	srv.limiter = newIPRateLimiter(RateLimitConfig{RPS: 1, Burst: 3})
	h := srv.Handler()

	// 1. Oversized query is rejected before DB/Hunspell work
	longWord := make([]byte, 150)
	for i := range longWord {
		longWord[i] = 'a'
	}
	reqLong := httptest.NewRequest(http.MethodGet, "/api/v1/analyze?q="+string(longWord), nil)
	reqLong.RemoteAddr = "203.0.113.10:12345"
	recLong := httptest.NewRecorder()
	h.ServeHTTP(recLong, reqLong)
	if recLong.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized query, got %d", recLong.Code)
	}

	// 2. Burst of 3 requests from a second IP succeeds, 4th immediately hits 429
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/hyphenate?q=pes", nil)
		req.RemoteAddr = "198.51.100.42:54321"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	reqBlocked := httptest.NewRequest(http.MethodGet, "/api/v1/hyphenate?q=pes", nil)
	reqBlocked.RemoteAddr = "198.51.100.42:54321"
	recBlocked := httptest.NewRecorder()
	h.ServeHTTP(recBlocked, reqBlocked)
	if recBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("4th burst request: expected 429, got %d", recBlocked.Code)
	}
	if recBlocked.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header on 429 response")
	}

	// 3. Spoofed X-Forwarded-For from a public RemoteAddr does NOT bypass limit
	reqSpoof := httptest.NewRequest(http.MethodGet, "/api/v1/hyphenate?q=pes", nil)
	reqSpoof.RemoteAddr = "198.51.100.42:54321"
	reqSpoof.Header.Set("X-Forwarded-For", "1.2.3.4")
	recSpoof := httptest.NewRecorder()
	h.ServeHTTP(recSpoof, reqSpoof)
	if recSpoof.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF from public peer should still be 429, got %d", recSpoof.Code)
	}

	// 4. /healthz is never throttled even when IP bucket is exhausted
	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	reqHealth.RemoteAddr = "198.51.100.42:54321"
	recHealth := httptest.NewRecorder()
	h.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Fatalf("/healthz should never be rate-limited, got %d", recHealth.Code)
	}
}
