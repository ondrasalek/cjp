package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ondrejsalek/cjp/internal/db"
	"github.com/ondrejsalek/cjp/internal/hunspell"
	"github.com/ondrejsalek/cjp/internal/hyphen"
	"github.com/ondrejsalek/cjp/internal/textnorm"
)

// Server serves the Czech Lexical REST API and optional SPA frontend.
type Server struct {
	store  *db.Store
	hyph   *hyphen.Hyphenator
	hun    *hunspell.Dictionary // optional (nil if running without Hunspell)
	webDir string
	mux    *http.ServeMux
}

// Config configures the HTTP API server.
type Config struct {
	Store      *db.Store
	Hyphenator *hyphen.Hyphenator
	Hunspell   *hunspell.Dictionary
	WebDir     string
}

// NewServer creates and wires a new API Server.
func NewServer(cfg Config) *Server {
	s := &Server{
		store:  cfg.Store,
		hyph:   cfg.Hyphenator,
		hun:    cfg.Hunspell,
		webDir: cfg.WebDir,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/meta", s.handleMeta)
	s.mux.HandleFunc("GET /api/v1/words/{word}", s.handleWordCard)
	s.mux.HandleFunc("GET /api/v1/analyze", s.handleAnalyze)
	s.mux.HandleFunc("GET /api/v1/autocomplete", s.handleAutocomplete)
	s.mux.HandleFunc("GET /api/v1/hyphenate", s.handleHyphenate)
	s.mux.HandleFunc("GET /api/v1/translations/{word}", s.handleTranslations)
	s.mux.HandleFunc("GET /api/v1/search", s.handleSearch)

	if s.webDir != "" {
		if st, err := os.Stat(s.webDir); err == nil && st.IsDir() {
			fs := http.FileServer(http.Dir(s.webDir))
			s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
				clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
				if clean != "." {
					full := filepath.Join(s.webDir, clean)
					if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
						fs.ServeHTTP(w, r)
						return
					}
				}
				http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
			})
		}
	}
}

// Handler returns the root http.Handler with CORS and JSON headers.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":          "cjp-api",
		"hunspell_enabled": s.hun != nil,
		"stats":            stats,
	})
}

// WordCardResponse is the response returned by GET /api/v1/words/{word}.
type WordCardResponse struct {
	Query           string            `json:"query"`
	ExactLemmaMatch bool              `json:"exact_lemma_match"`
	ASCIIFallback   bool              `json:"ascii_fallback,omitempty"`
	Hyphenation     string            `json:"hyphenation,omitempty"`
	Syllables       []string          `json:"syllables,omitempty"`
	FormMatches     []db.FormMatch    `json:"form_matches,omitempty"`
	Entries         []*db.LexemeDetail `json:"entries"`
}

func (s *Server) handleWordCard(w http.ResponseWriter, r *http.Request) {
	word := strings.TrimSpace(r.PathValue("word"))
	if word == "" {
		writeError(w, http.StatusBadRequest, "missing word parameter")
		return
	}

	ctx := r.Context()
	langs := parseLangsParam(r.URL.Query().Get("langs"))
	expandHunspell := r.URL.Query().Get("expand") != "false"

	// 1. Exact diacritic lookup first
	lexIDs, err := s.store.FindLexemeIDsByLemma(ctx, word, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	exactLemma := len(lexIDs) > 0

	formMatches, err := s.store.FindFormMatches(ctx, word, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Merge lexeme IDs from morphological form matches
	seenID := make(map[int64]bool, len(lexIDs))
	for _, id := range lexIDs {
		seenID[id] = true
	}
	for _, fm := range formMatches {
		if !seenID[fm.LexemeID] {
			seenID[fm.LexemeID] = true
			lexIDs = append(lexIDs, fm.LexemeID)
		}
	}

	// Also check Hunspell stemmer for inflected forms not in Wiktionary/UniMorph
	if s.hun != nil {
		stems := s.hun.Analyze(word)
		for _, st := range stems {
			stIDs, err := s.store.FindLexemeIDsByLemma(ctx, st, false)
			if err == nil {
				for _, id := range stIDs {
					if !seenID[id] {
						seenID[id] = true
						lexIDs = append(lexIDs, id)
					}
					hasFM := false
					for _, fm := range formMatches {
						if fm.LexemeID == id && strings.EqualFold(fm.Form, word) {
							hasFM = true
							break
						}
					}
					if !hasFM {
						formMatches = append(formMatches, db.FormMatch{
							Form:     word,
							LexemeID: id,
							Lemma:    st,
							Source:   "hunspell_cs",
						})
					}
				}
			}
		}
	}

	// 2. ASCII fallback if user omitted diacritics (e.g. /words/zena)
	asciiFallback := false
	if len(lexIDs) == 0 && textnorm.NormalizeWord(word) == textnorm.StripDiacritics(word) {
		asciiIDs, err := s.store.FindLexemeIDsByLemma(ctx, word, true)
		if err == nil && len(asciiIDs) > 0 {
			asciiFallback = true
			for _, id := range asciiIDs {
				if !seenID[id] {
					seenID[id] = true
					lexIDs = append(lexIDs, id)
				}
			}
		} else {
			asciiFMs, err := s.store.FindFormMatches(ctx, word, true)
			if err == nil && len(asciiFMs) > 0 {
				asciiFallback = true
				formMatches = append(formMatches, asciiFMs...)
				for _, fm := range asciiFMs {
					if !seenID[fm.LexemeID] {
						seenID[fm.LexemeID] = true
						lexIDs = append(lexIDs, fm.LexemeID)
					}
				}
			}
		}
	}

	if len(lexIDs) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":       "word not found",
			"query":       word,
			"hyphenation": s.hyphenateWord(word),
			"syllables":   s.hyphenateParts(word),
		})
		return
	}

	var stemmer db.StemmerFunc
	if s.hun != nil {
		stemmer = s.hun.Analyze
	}

	entries := make([]*db.LexemeDetail, 0, len(lexIDs))
	for _, id := range lexIDs {
		det, err := s.store.GetLexemeDetail(ctx, id, langs)
		if err != nil {
			continue
		}
		if expandHunspell && len(det.Forms) == 0 && s.hun != nil {
			gen := s.hun.Expand(det.Lemma)
			if len(gen) > 0 {
				det.GeneratedForms = gen
			}
		}
		s.store.EnrichFallbackTranslations(ctx, det, langs, stemmer)
		entries = append(entries, det)
	}

	hyphStr := s.hyphenateWord(word)
	sylls := s.hyphenateParts(word)
	if exactLemma && len(entries) > 0 && entries[0].Hyphenation != "" {
		hyphStr = entries[0].Hyphenation
		sylls = entries[0].Syllables
	}

	writeJSON(w, http.StatusOK, WordCardResponse{
		Query:           word,
		ExactLemmaMatch: exactLemma,
		ASCIIFallback:   asciiFallback,
		Hyphenation:     hyphStr,
		Syllables:       sylls,
		FormMatches:     formMatches,
		Entries:         entries,
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("word"))
	}
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing 'q' query parameter")
		return
	}

	ctx := r.Context()
	lemmaIDs, err := s.store.FindLexemeIDsByLemma(ctx, q, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	formMatches, err := s.store.FindFormMatches(ctx, q, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	lemmas := make([]string, 0, 4)
	addLemma := func(l string) {
		if l != "" && !slices.Contains(lemmas, l) {
			lemmas = append(lemmas, l)
		}
	}
	for _, fm := range formMatches {
		addLemma(fm.Lemma)
	}
	for _, id := range lemmaIDs {
		if det, err := s.store.GetLexemeDetail(ctx, id, nil); err == nil {
			addLemma(det.Lemma)
		}
	}
	if s.hun != nil {
		for _, st := range s.hun.Analyze(q) {
			addLemma(st)
		}
	}

	valid := len(lemmaIDs) > 0 || len(formMatches) > 0 || len(lemmas) > 0
	writeJSON(w, http.StatusOK, map[string]any{
		"query":       q,
		"valid":       valid,
		"is_lemma":    len(lemmaIDs) > 0,
		"lemmas":      lemmas,
		"hyphenation": s.hyphenateWord(q),
		"syllables":   s.hyphenateParts(q),
		"morphology":  formMatches,
	})
}

func (s *Server) handleAutocomplete(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing 'q' query parameter")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	items, err := s.store.Autocomplete(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":       q,
		"count":       len(items),
		"suggestions": items,
	})
}

func (s *Server) handleHyphenate(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("word"))
	}
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing 'q' query parameter")
		return
	}
	sep := r.URL.Query().Get("sep")
	if sep == "" {
		sep = "-"
	}

	// Prefer dictionary syllable division if exact lemma exists in DB
	parts := s.hyphenateParts(q)
	source := "liang_hyph_cs"
	if ids, err := s.store.FindLexemeIDsByLemma(r.Context(), q, false); err == nil && len(ids) > 0 {
		if det, err := s.store.GetLexemeDetail(r.Context(), ids[0], nil); err == nil && len(det.Syllables) > 0 {
			parts = det.Syllables
			source = det.Source
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":       q,
		"hyphenated":  strings.Join(parts, sep),
		"syllables":   parts,
		"syllable_count": len(parts),
		"source":      source,
	})
}

func (s *Server) handleTranslations(w http.ResponseWriter, r *http.Request) {
	word := strings.TrimSpace(r.PathValue("word"))
	if word == "" {
		writeError(w, http.StatusBadRequest, "missing word parameter")
		return
	}
	langs := parseLangsParam(r.URL.Query().Get("langs"))
	if len(langs) == 0 {
		langs = []string{"*"}
	}

	var stemmer db.StemmerFunc
	if s.hun != nil {
		stemmer = s.hun.Analyze
	}

	ctx := r.Context()
	lexIDs, err := s.store.FindLexemeIDsByLemma(ctx, word, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(lexIDs) == 0 {
		fms, _ := s.store.FindFormMatches(ctx, word, true)
		for _, fm := range fms {
			if !slices.Contains(lexIDs, fm.LexemeID) {
				lexIDs = append(lexIDs, fm.LexemeID)
			}
		}
	}
	if len(lexIDs) == 0 && s.hun != nil {
		for _, st := range s.hun.Analyze(word) {
			stIDs, err := s.store.FindLexemeIDsByLemma(ctx, st, false)
			if err == nil {
				for _, id := range stIDs {
					if !slices.Contains(lexIDs, id) {
						lexIDs = append(lexIDs, id)
					}
				}
			}
		}
	}
	if len(lexIDs) == 0 {
		lexIDs, _ = s.store.FindLexemeIDsByLemma(ctx, word, true)
	}
	if len(lexIDs) == 0 {
		writeError(w, http.StatusNotFound, "word not found")
		return
	}

	type lexTranslationGroup struct {
		LexemeID              int64            `json:"lexeme_id"`
		Lemma                 string           `json:"lemma"`
		POS                   string           `json:"pos"`
		POSCs                 string           `json:"pos_cs,omitempty"`
		TranslationsFromLemma string           `json:"translations_from_lemma,omitempty"`
		Translations          []db.Translation `json:"translations"`
	}

	var results []lexTranslationGroup
	for _, id := range lexIDs {
		det, err := s.store.GetLexemeDetail(ctx, id, langs)
		if err != nil {
			continue
		}
		s.store.EnrichFallbackTranslations(ctx, det, langs, stemmer)
		if len(det.Translations) > 0 {
			results = append(results, lexTranslationGroup{
				LexemeID:              det.ID,
				Lemma:                 det.Lemma,
				POS:                   det.POS,
				POSCs:                 det.POSCs,
				TranslationsFromLemma: det.TranslationsFromLemma,
				Translations:          det.Translations,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":   word,
		"langs":   langs,
		"results": results,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing 'q' query parameter")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	res, err := s.store.SearchDefinitionsWithMeta(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   q,
		"count":   len(res.Hits),
		"relaxed": res.Relaxed,
		"hits":    res.Hits,
	})
}

func (s *Server) hyphenateParts(w string) []string {
	if s.hyph == nil {
		return []string{w}
	}
	return s.hyph.Parts(w)
}

func (s *Server) hyphenateWord(w string) string {
	if s.hyph == nil {
		return w
	}
	return s.hyph.Hyphenate(w, "-")
}

func parseLangsParam(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if raw == "*" || strings.EqualFold(raw, "all") {
		return []string{"*"}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": msg,
	})
}
