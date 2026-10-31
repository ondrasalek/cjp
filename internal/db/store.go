package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ondrejsalek/cjp/internal/textnorm"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite lexical database.
type Store struct {
	db *sql.DB
}

// Open opens an existing or new SQLite database at path.
func Open(path string, readOnly bool) (*Store, error) {
	dsn := path
	if readOnly {
		dsn = fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)&_pragma=cache_size(-64000)", path)
	} else {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=cache_size(-128000)", path)
	}

	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if readOnly {
		sqldb.SetMaxOpenConns(16)
	} else {
		sqldb.SetMaxOpenConns(1)
	}
	if err := sqldb.Ping(); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Store{db: sqldb}, nil
}

// DB exposes the underlying *sql.DB for batch ETL imports.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// InitSchema creates tables (call Indexes after bulk loading for maximum speed).
func (s *Store) InitSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, SchemaSQL); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

// CreateIndexes builds secondary lookup indexes and optimizes the SQLite query planner.
func (s *Store) CreateIndexes(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, IndexesSQL); err != nil {
		return fmt.Errorf("exec indexes: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `ANALYZE;`)
	return nil
}

// Stats summarizes database row counts.
type Stats struct {
	Lexemes      int64            `json:"lexemes"`
	WordForms    int64            `json:"word_forms"`
	Senses       int64            `json:"senses"`
	Relations    int64            `json:"relations"`
	Translations int64            `json:"translations"`
	Sources      []DataSourceInfo `json:"sources"`
}

// GetStats returns aggregate table counts and data source provenance.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	st := &Stats{}
	queries := []struct {
		q   string
		dst *int64
	}{
		{"SELECT COUNT(*) FROM lexemes", &st.Lexemes},
		{"SELECT COUNT(*) FROM word_forms", &st.WordForms},
		{"SELECT COUNT(*) FROM senses", &st.Senses},
		{"SELECT COUNT(*) FROM relations", &st.Relations},
		{"SELECT COUNT(*) FROM translations", &st.Translations},
	}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.q).Scan(item.dst); err != nil {
			return nil, err
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, license, url, attribution, record_count, imported_at
		FROM data_sources ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ds DataSourceInfo
		if err := rows.Scan(&ds.ID, &ds.Name, &ds.License, &ds.URL, &ds.Attribution, &ds.RecordCount, &ds.ImportedAt); err != nil {
			return nil, err
		}
		st.Sources = append(st.Sources, ds)
	}
	return st, rows.Err()
}

// FindLexemeIDsByLemma finds lexeme IDs matching lemma (case-insensitive, with optional ASCII fallback).
func (s *Store) FindLexemeIDsByLemma(ctx context.Context, word string, asciiFallback bool) ([]int64, error) {
	low := textnorm.NormalizeWord(word)
	if low == "" {
		return nil, nil
	}
	ids, err := s.queryIDs(ctx, `SELECT id FROM lexemes WHERE lemma_lower = ? ORDER BY priority DESC, (lemma = ?) DESC, id ASC`, low, word)
	if err != nil || len(ids) > 0 || !asciiFallback {
		return ids, err
	}
	asc := textnorm.StripDiacritics(word)
	return s.queryIDs(ctx, `SELECT id FROM lexemes WHERE lemma_ascii = ? ORDER BY priority DESC, id ASC LIMIT 10`, asc)
}

func (s *Store) queryIDs(ctx context.Context, q string, args ...any) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindFormMatches finds morphological form matches for a given surface word.
func (s *Store) FindFormMatches(ctx context.Context, word string, asciiFallback bool) ([]FormMatch, error) {
	low := textnorm.NormalizeWord(word)
	if low == "" {
		return nil, nil
	}
	matches, err := s.queryFormMatches(ctx, `w.form_lower = ?`, low)
	if err != nil || len(matches) > 0 || !asciiFallback {
		return matches, err
	}
	asc := textnorm.StripDiacritics(word)
	return s.queryFormMatches(ctx, `w.form_ascii = ?`, asc)
}

func (s *Store) queryFormMatches(ctx context.Context, where string, arg string) ([]FormMatch, error) {
	q := fmt.Sprintf(`
		SELECT w.form, w.lexeme_id, l.lemma, l.pos, l.pos_cs,
		       w.gram_case, w.number, w.person, w.tense, w.mood, w.voice,
		       w.degree, w.gender, w.animacy, w.nonfinite, w.raw_tags, w.source
		FROM word_forms w
		JOIN lexemes l ON l.id = w.lexeme_id
		WHERE %s
		ORDER BY l.priority DESC, l.id ASC, w.gram_case ASC, w.number ASC
		LIMIT 60
	`, where)

	rows, err := s.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FormMatch
	for rows.Next() {
		var fm FormMatch
		var rawTags string
		if err := rows.Scan(
			&fm.Form, &fm.LexemeID, &fm.Lemma, &fm.POS, &fm.POSCs,
			&fm.Case, &fm.Number, &fm.Person, &fm.Tense, &fm.Mood, &fm.Voice,
			&fm.Degree, &fm.Gender, &fm.Animacy, &fm.NonFinite, &rawTags, &fm.Source,
		); err != nil {
			return nil, err
		}
		fm.CaseName = CaseName(fm.Case)
		if rawTags != "" {
			fm.Tags = strings.Split(rawTags, ",")
		}
		out = append(out, fm)
	}
	return out, rows.Err()
}

// GetLexemeDetail loads the full dictionary card for a lexeme ID.
func (s *Store) GetLexemeDetail(ctx context.Context, id int64, langs []string) (*LexemeDetail, error) {
	var d LexemeDetail
	err := s.db.QueryRowContext(ctx, `
		SELECT id, lemma, pos, pos_cs, gender, animacy, aspect, ipa, audio_url, hyphenation, etymology, source
		FROM lexemes WHERE id = ?
	`, id).Scan(
		&d.ID, &d.Lemma, &d.POS, &d.POSCs, &d.Gender, &d.Animacy, &d.Aspect,
		&d.IPA, &d.AudioURL, &d.Hyphenation, &d.Etymology, &d.Source,
	)
	if err != nil {
		return nil, err
	}
	if d.Hyphenation != "" {
		d.Syllables = strings.Split(d.Hyphenation, "-")
	}

	// 1. Senses
	sRows, err := s.db.QueryContext(ctx, `
		SELECT sense_index, definition, labels, examples_json, source
		FROM senses WHERE lexeme_id = ? ORDER BY sense_index ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer sRows.Close()
	for sRows.Next() {
		var sn Sense
		var labelsStr, exJSON string
		if err := sRows.Scan(&sn.SenseIndex, &sn.Definition, &labelsStr, &exJSON, &sn.Source); err != nil {
			return nil, err
		}
		if labelsStr != "" {
			sn.Labels = strings.Split(labelsStr, ",")
		}
		if exJSON != "" && exJSON != "[]" {
			_ = json.Unmarshal([]byte(exJSON), &sn.Examples)
		}
		d.Senses = append(d.Senses, sn)
	}
	if err := sRows.Err(); err != nil {
		return nil, err
	}

	// 2. Relations (synonyms, antonyms, related)
	rRows, err := s.db.QueryContext(ctx, `
		SELECT sense_index, rel_type, target_word, qualifier, source
		FROM relations WHERE lexeme_id = ? ORDER BY sense_index ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rRows.Close()
	for rRows.Next() {
		var r Relation
		if err := rRows.Scan(&r.SenseIndex, &r.Type, &r.Word, &r.Qualifier, &r.Source); err != nil {
			return nil, err
		}
		switch r.Type {
		case "synonym":
			d.Synonyms = append(d.Synonyms, r)
		case "antonym":
			d.Antonyms = append(d.Antonyms, r)
		default:
			d.Related = append(d.Related, r)
		}
	}
	if err := rRows.Err(); err != nil {
		return nil, err
	}

	// 3. Word Forms
	fRows, err := s.db.QueryContext(ctx, `
		SELECT form, gram_case, number, person, tense, mood, voice, degree, gender, animacy, nonfinite, raw_tags, source
		FROM word_forms WHERE lexeme_id = ?
		ORDER BY gram_case ASC, number ASC, person ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer fRows.Close()
	for fRows.Next() {
		var wf WordForm
		var rawTags string
		if err := fRows.Scan(
			&wf.Form, &wf.Case, &wf.Number, &wf.Person, &wf.Tense, &wf.Mood,
			&wf.Voice, &wf.Degree, &wf.Gender, &wf.Animacy, &wf.NonFinite, &rawTags, &wf.Source,
		); err != nil {
			return nil, err
		}
		wf.CaseName = CaseName(wf.Case)
		if rawTags != "" {
			wf.Tags = strings.Split(rawTags, ",")
		}
		d.Forms = append(d.Forms, wf)
	}
	if err := fRows.Err(); err != nil {
		return nil, err
	}

	if len(d.Forms) > 0 {
		d.Inflection = BuildInflectionSummary(d.Lemma, d.POS, d.Forms)
	}

	// 4. Translations (optional language filter; pass ["*"] for all languages)
	if len(langs) > 0 {
		translations, err := s.GetTranslationsByLexeme(ctx, id, langs)
		if err != nil {
			return nil, err
		}
		d.Translations = translations
	}

	return &d, nil
}

// GetTranslationsByLexeme returns translations for a lexeme filtered by language codes (or all if langs contains "*").
func (s *Store) GetTranslationsByLexeme(ctx context.Context, lexemeID int64, langs []string) ([]Translation, error) {
	all := slices.Contains(langs, "*")
	var rows *sql.Rows
	var err error
	if all || len(langs) == 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT sense_index, sense_gloss, lang_code, lang_name, target_word, qualifier, source
			FROM translations WHERE lexeme_id = ?
			ORDER BY sense_index ASC, lang_code ASC, id ASC
		`, lexemeID)
	} else {
		placeholders := make([]string, len(langs))
		args := make([]any, 0, len(langs)+1)
		args = append(args, lexemeID)
		for i, l := range langs {
			placeholders[i] = "?"
			args = append(args, strings.ToLower(strings.TrimSpace(l)))
		}
		q := fmt.Sprintf(`
			SELECT sense_index, sense_gloss, lang_code, lang_name, target_word, qualifier, source
			FROM translations
			WHERE lexeme_id = ? AND lang_code IN (%s)
			ORDER BY sense_index ASC, lang_code ASC, id ASC
		`, strings.Join(placeholders, ","))
		rows, err = s.db.QueryContext(ctx, q, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Translation
	for rows.Next() {
		var tr Translation
		if err := rows.Scan(&tr.SenseIndex, &tr.SenseGloss, &tr.LangCode, &tr.LangName, &tr.Word, &tr.Qualifier, &tr.Source); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

// Autocomplete returns prefix suggestions (supporting both diacritic and ASCII queries).
func (s *Store) Autocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	low := textnorm.NormalizeWord(prefix)
	if low == "" {
		return nil, nil
	}
	asc := textnorm.StripDiacritics(prefix)
	hasDiacritics := (low != asc)

	var col, val string
	if hasDiacritics {
		col = "l.lemma_lower"
		val = low
	} else {
		col = "l.lemma_ascii"
		val = asc
	}

	// Range scan >= val AND < nextPrefix uses B-tree index cleanly without LIKE wildcard issues
	upperBound := val + "\uffff"
	q := fmt.Sprintf(`
		SELECT l.lemma, l.pos, l.pos_cs, l.hyphenation, l.source,
		       COALESCE((SELECT definition FROM senses s WHERE s.lexeme_id = l.id ORDER BY s.sense_index ASC LIMIT 1), '')
		FROM lexemes l
		WHERE %s >= ? AND %s < ?
		ORDER BY (l.lemma_lower = ?) DESC, (%s = ?) DESC, l.priority DESC, LENGTH(l.lemma) ASC, l.lemma ASC
		LIMIT ?
	`, col, col, col)

	rows, err := s.db.QueryContext(ctx, q, val, upperBound, low, val, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []AutocompleteItem
	for rows.Next() {
		var it AutocompleteItem
		if err := rows.Scan(&it.Lemma, &it.POS, &it.POSCs, &it.Hyphenation, &it.Source, &it.Preview); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SearchResult wraps FTS5 definition hits with metadata on whether query relaxation was used.
type SearchResult struct {
	Hits    []SearchHit `json:"hits"`
	Relaxed bool        `json:"relaxed,omitempty"`
}

var czechFTSStopwords = map[string]bool{
	"a": true, "i": true, "o": true, "u": true, "v": true, "ve": true,
	"s": true, "se": true, "z": true, "ze": true, "k": true, "ke": true, "ku": true,
	"na": true, "do": true, "od": true, "po": true, "za": true, "pro": true,
	"nad": true, "pod": true, "bez": true, "pri": true, "pred": true, "mezi": true,
	"nebo": true, "jako": true, "ktery": true, "ktera": true, "ktere": true,
	"jenz": true, "co": true, "to": true, "je": true, "jsou": true, "byt": true,
}

var czechInflectionSuffixes = []string{
	"ach", "ech", "ich", "ami", "emi", "ovi", "ove", "ych", "ymi", "eho", "emu",
	"ou", "em", "um", "im", "mi",
	"u", "y", "e", "i", "a", "o",
}

func czechStemPrefix(word string) string {
	asc := textnorm.StripDiacritics(word)
	runes := []rune(asc)
	if len(runes) < 5 {
		return ""
	}
	for _, suf := range czechInflectionSuffixes {
		sr := []rune(suf)
		if len(runes)-len(sr) >= 4 && strings.HasSuffix(asc, suf) {
			return string(runes[:len(runes)-len(sr)])
		}
	}
	return ""
}

func quoteFTSToken(tok string) string {
	clean := strings.ReplaceAll(tok, `"`, `""`)
	return `"` + clean + `"*`
}

// SearchDefinitions runs a full-text FTS5 query across lemmas and definitions.
func (s *Store) SearchDefinitions(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	res, err := s.SearchDefinitionsWithMeta(ctx, query, limit)
	return res.Hits, err
}

// SearchDefinitionsWithMeta runs an FTS5 search with Czech stopword filtering, morphological
// lemma/stem expansion, and a ranked BM25 OR fallback when no single definition contains every word.
func (s *Store) SearchDefinitionsWithMeta(ctx context.Context, query string, limit int) (SearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rawFields := strings.Fields(query)
	var allTokens []string
	var contentTokens []string
	for _, f := range rawFields {
		tok := strings.Trim(f, ".,;:!?\"'()[]«»„“”–—-")
		if tok == "" {
			continue
		}
		allTokens = append(allTokens, tok)
		if !czechFTSStopwords[textnorm.StripDiacritics(tok)] {
			contentTokens = append(contentTokens, tok)
		}
	}
	if len(allTokens) == 0 {
		return SearchResult{}, nil
	}
	activeTokens := contentTokens
	if len(activeTokens) == 0 {
		activeTokens = allTokens
	}
	if len(activeTokens) > 8 {
		activeTokens = activeTokens[:8]
	}

	groups := make([]string, 0, len(activeTokens))
	for _, tok := range activeTokens {
		var variants []string
		addVariant := func(v string) {
			v = strings.TrimSpace(v)
			if v == "" || strings.ContainsAny(v, " \t") {
				return
			}
			q := quoteFTSToken(v)
			if !slices.Contains(variants, q) {
				variants = append(variants, q)
			}
		}

		addVariant(tok)
		if stem := czechStemPrefix(tok); stem != "" {
			addVariant(stem)
		}
		if fms, err := s.FindFormMatches(ctx, tok, true); err == nil {
			addedLemmas := 0
			for _, fm := range fms {
				if addedLemmas >= 3 {
					break
				}
				if fm.Lemma != "" && !strings.EqualFold(fm.Lemma, tok) {
					addVariant(fm.Lemma)
					if lStem := czechStemPrefix(fm.Lemma); lStem != "" {
						addVariant(lStem)
					}
					addedLemmas++
				}
			}
		}

		if len(variants) == 1 {
			groups = append(groups, variants[0])
		} else if len(variants) > 1 {
			groups = append(groups, "("+strings.Join(variants, " OR ")+")")
		}
	}
	if len(groups) == 0 {
		return SearchResult{}, nil
	}

	hits, err := s.runFTSQuery(ctx, strings.Join(groups, " AND "), limit)
	if err != nil {
		return SearchResult{}, err
	}
	if len(hits) > 0 || len(groups) < 2 {
		return SearchResult{Hits: hits}, nil
	}

	// Ranked BM25 OR fallback when no single definition contains every query group verbatim
	orHits, err := s.runFTSQuery(ctx, strings.Join(groups, " OR "), limit)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{
		Hits:    orHits,
		Relaxed: len(orHits) > 0,
	}, nil
}

func (s *Store) runFTSQuery(ctx context.Context, ftsExpr string, limit int) ([]SearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.lexeme_id, l.lemma, l.pos, l.pos_cs, f.definition
		FROM senses_fts f
		JOIN lexemes l ON l.id = f.lexeme_id
		WHERE senses_fts MATCH ?
		ORDER BY bm25(senses_fts)
		LIMIT ?
	`, ftsExpr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.LexemeID, &h.Lemma, &h.POS, &h.POSCs, &h.Definition); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// BuildInflectionSummary organizes flat WordForms into clean declension/conjugation tables.
func BuildInflectionSummary(lemma, pos string, forms []WordForm) *InflectionSummary {
	sum := &InflectionSummary{}
	appendUniq := func(list []string, val string) []string {
		if val == "" || slices.Contains(list, val) {
			return list
		}
		return append(list, val)
	}

	// 1. Declension (cases 1..7)
	sgByCase := make(map[string][]string)
	plByCase := make(map[string][]string)
	hasDecl := false
	for _, wf := range forms {
		if wf.Case >= "1" && wf.Case <= "7" {
			// For adjectives, keep base positive degree in primary declension table if degree is specified
			if wf.Degree != "" && wf.Degree != "1" {
				continue
			}
			hasDecl = true
			switch wf.Number {
			case "sg":
				sgByCase[wf.Case] = appendUniq(sgByCase[wf.Case], wf.Form)
			case "pl", "du":
				plByCase[wf.Case] = appendUniq(plByCase[wf.Case], wf.Form)
			}
		}
	}
	if hasDecl {
		if len(sgByCase["1"]) == 0 && (pos == "noun" || pos == "adj" || pos == "pron" || pos == "num") {
			sgByCase["1"] = []string{lemma}
		}
		for _, c := range []string{"1", "2", "3", "4", "5", "6", "7"} {
			if len(sgByCase[c]) > 0 || len(plByCase[c]) > 0 {
				sum.Declension = append(sum.Declension, CaseRow{
					Case:     c,
					CaseName: CaseName(c),
					Singular: sgByCase[c],
					Plural:   plByCase[c],
				})
			}
		}
	}

	// 2. Degrees of comparison (adjectives & adverbs)
	degMap := make(map[string][]string)
	for _, wf := range forms {
		if wf.Degree == "2" || wf.Degree == "3" {
			if wf.Case == "" || wf.Case == "1" {
				degMap[wf.Degree] = appendUniq(degMap[wf.Degree], wf.Form)
			}
		}
	}
	if len(degMap) > 0 {
		degMap["1"] = []string{lemma}
		sum.Degrees = degMap
	}

	// 3. Verb conjugation
	if pos == "verb" {
		conj := &ConjugationTable{
			Infinitive:      []string{lemma},
			PresentOrFuture: make(map[string][]string),
			Imperative:      make(map[string][]string),
		}
		hasConj := false
		for _, wf := range forms {
			if wf.NonFinite == "inf" {
				conj.Infinitive = appendUniq(conj.Infinitive, wf.Form)
			} else if wf.Mood == "imp" && wf.Person != "" && wf.Number != "" {
				key := wf.Person + "." + wf.Number
				conj.Imperative[key] = appendUniq(conj.Imperative[key], wf.Form)
				hasConj = true
			} else if (wf.Mood == "ind" || wf.Mood == "") && (wf.Tense == "pres" || wf.Tense == "fut") && wf.Person != "" && wf.Number != "" {
				key := wf.Person + "." + wf.Number
				conj.PresentOrFuture[key] = appendUniq(conj.PresentOrFuture[key], wf.Form)
				hasConj = true
			} else if wf.NonFinite == "part" {
				if wf.Voice == "pass" {
					conj.PassParticiple = appendUniq(conj.PassParticiple, wf.Form)
				} else {
					conj.PastParticiple = appendUniq(conj.PastParticiple, wf.Form)
				}
				hasConj = true
			} else if wf.NonFinite == "transgr" {
				conj.Transgressive = appendUniq(conj.Transgressive, wf.Form)
				hasConj = true
			}
		}
		if len(conj.PresentOrFuture) == 0 {
			conj.PresentOrFuture = nil
		}
		if len(conj.Imperative) == 0 {
			conj.Imperative = nil
		}
		if hasConj {
			sum.Conjugation = conj
		}
	}

	if len(sum.Declension) == 0 && sum.Conjugation == nil && len(sum.Degrees) == 0 {
		return nil
	}
	return sum
}
