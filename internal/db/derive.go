package db

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ondrejsalek/cjp/internal/textnorm"
)

// StemmerFunc is an optional morphological stemmer (such as Hunspell Analyze).
type StemmerFunc func(word string) []string

var irregularBases = map[string][]string{
	"lepší":     {"dobrý"},
	"líp":       {"dobře", "dobrý"},
	"lépe":      {"dobře", "dobrý"},
	"horší":     {"špatný", "zlý"},
	"hůř":       {"špatně", "špatný", "zlý"},
	"hůře":      {"špatně", "špatný", "zlý"},
	"větší":     {"velký", "veliký"},
	"víc":       {"mnoho", "hodně"},
	"více":      {"mnoho", "hodně"},
	"menší":     {"malý"},
	"míň":       {"málo"},
	"méně":      {"málo"},
	"delší":     {"dlouhý"},
	"déle":      {"dlouho", "dlouhý"},
	"dýl":       {"dlouho", "dlouhý"},
	"kratší":    {"krátký"},
	"krátce":    {"krátký"},
	"vyšší":     {"vysoký"},
	"výš":       {"vysoko", "vysoký"},
	"výše":      {"vysoko", "vysoký"},
	"nižší":     {"nízký"},
	"níž":       {"nízko", "nízký"},
	"níže":      {"nízko", "nízký"},
	"širší":     {"široký"},
	"šíř":       {"široko", "široký"},
	"šíře":      {"široko", "široký"},
	"užší":      {"úzký"},
	"úže":       {"úzký"},
	"úžeji":     {"úzký"},
	"hlubší":    {"hluboký"},
	"hlouběji":  {"hluboký"},
	"bližší":    {"blízký"},
	"blíž":      {"blízko", "blízký"},
	"blíže":     {"blízko", "blízký"},
	"dražší":    {"drahý"},
	"dráž":      {"draze", "drahý"},
	"dráze":     {"drahý"},
	"tišší":     {"tichý"},
	"tišeji":    {"tichý"},
	"tiše":      {"tichý"},
	"sušší":     {"suchý"},
	"sušeji":    {"suchý"},
	"suše":      {"suchý"},
	"lehčí":     {"lehký"},
	"lehčeji":   {"lehký"},
	"lehce":     {"lehký"},
	"měkčí":     {"měkký"},
	"měkčeji":   {"měkký"},
	"měkce":     {"měkký"},
	"tenčí":     {"tenký"},
	"tenčeji":   {"tenký"},
	"těžší":     {"těžký"},
	"tíže":      {"těžce", "těžký"},
	"těžce":     {"těžký"},
	"snazší":    {"snadný"},
	"snadnější": {"snadný"},
	"snáz":      {"snadno", "snadný"},
	"snáze":     {"snadno", "snadný"},
	"dřívější":  {"brzký", "brzy"},
	"dřív":      {"brzy"},
	"dříve":     {"brzy"},
	"pozdější":  {"pozdní", "pozdě"},
	"později":   {"pozdě", "pozdní"},
	"hezčí":     {"hezký"},
}

type suffixRule struct {
	suffix       string
	replacements []string
	minStemRunes int
}

var (
	// Comparative & adverbial rules run BEFORE negation `ne-` stripping so
	// negated comparatives like `nešťastnější` match `nešťastný` first if translated.
	inflectionAndAdverbRules = []suffixRule{
		// Comparative adjectives & adverbs with palatalization
		{"čtější", []string{"cký", "cky"}, 2},
		{"čtěji", []string{"cký", "cky"}, 2},
		{"štější", []string{"ský", "sky"}, 2},
		{"štěji", []string{"ský", "sky"}, 2},
		{"nější", []string{"ní", "ný", "ně"}, 2},
		{"něji", []string{"ní", "ný", "ně"}, 2},
		{"řejší", []string{"rý"}, 2},
		{"řeji", []string{"rý"}, 2},
		{"čejší", []string{"ký"}, 2},
		{"čeji", []string{"ký"}, 2},
		{"žejší", []string{"hý"}, 2},
		{"žeji", []string{"hý"}, 2},
		{"šejší", []string{"chý"}, 2},
		{"šeji", []string{"chý"}, 2},
		{"ější", []string{"ý", "í"}, 2},
		{"ěji", []string{"ý", "í"}, 2},
		{"ejší", []string{"ý", "í"}, 2},
		{"eji", []string{"ý", "í"}, 2},
		{"ší", []string{"ý", "ký", "oký", "eký"}, 3},
		{"čí", []string{"ký", "kký"}, 3},

		// Derived adverbs -> positive adjectives
		{"cky", []string{"cký"}, 2},
		{"sky", []string{"ský"}, 2},
		{"ně", []string{"ní", "ný"}, 2},
		{"ře", []string{"rý"}, 2},
		{"ce", []string{"ký", "oký"}, 3},
		{"ze", []string{"hý", "zký"}, 3},
		{"še", []string{"chý"}, 2},
		{"le", []string{"lý"}, 3},
		{"ě", []string{"ý", "í"}, 3},
		{"o", []string{"ý"}, 4},
	}

	// Broader derivational suffixes (deverbal, abstract nouns, possessives, feminine agents, relational adj/verbs)
	derivationalRules = []suffixRule{
		// Verbal nouns
		{"ování", []string{"ovat"}, 3},
		{"ávání", []string{"ávat", "at"}, 3},
		{"ání", []string{"at", "át"}, 3},
		{"ění", []string{"ět", "it"}, 3},
		{"ení", []string{"it", "et", "out"}, 3},
		{"tí", []string{"t"}, 2},

		// Deverbal adjectives & participles
		{"ovávatelný", []string{"ovávat", "ovat"}, 3},
		{"ovatelný", []string{"ovat"}, 3},
		{"ávatelný", []string{"ávat", "at"}, 3},
		{"atelný", []string{"at"}, 3},
		{"itelný", []string{"it", "ět", "et"}, 3},
		{"ující", []string{"ovat"}, 3},
		{"ající", []string{"at"}, 3},
		{"ící", []string{"it", "ět", "et"}, 3},
		{"ovaný", []string{"ovat"}, 3},

		// Abstract nouns in -ost
		{"nost", []string{"ní", "ný"}, 2},
		{"ost", []string{"ý", "í"}, 3},

		// Possessives
		{"čin", []string{"ka"}, 3},
		{"ův", []string{""}, 3},
		{"in", []string{"a", "e"}, 3},

		// Feminine agent nouns
		{"telka", []string{"tel"}, 3},
		{"istka", []string{"ista"}, 3},
		{"ářka", []string{"ář"}, 3},
		{"ařka", []string{"ař"}, 3},
		{"nice", []string{"ník"}, 3},
		{"ka", []string{""}, 4},

		// Relational verbs & adjectives -> base nouns
		{"ovat", []string{"", "a", "o"}, 3},
		{"ňský", []string{"ně", "ň"}, 3},
		{"ský", []string{"", "a", "e", "o", "ie"}, 3},
		{"cký", []string{"", "a", "e", "ie"}, 3},
		{"ový", []string{"", "a", "o", "e"}, 3},
		{"ní", []string{"", "a", "e", "o", "ie"}, 3},
		{"ný", []string{"", "a", "e", "o"}, 3},
	}
)

// CandidateBaseWords returns ordered candidate base lemmas for a Czech word
// using productive morphological rules (superlative/comparative, adverbs, negation, derivation).
func CandidateBaseWords(word string) []string {
	w := textnorm.NormalizeWord(word)
	if w == "" {
		return nil
	}

	var out []string
	add := func(cand string) {
		cand = strings.TrimSpace(cand)
		if cand != "" && cand != w && !slices.Contains(out, cand) {
			out = append(out, cand)
		}
	}

	// 1. Irregular comparatives/adverbs
	if irr, ok := irregularBases[w]; ok {
		for _, b := range irr {
			add(b)
		}
	}

	// 2. Superlative prefix `nej-` (does not change polarity)
	if strings.HasPrefix(w, "nej") {
		rest := strings.TrimPrefix(w, "nej")
		if utf8.RuneCountInString(rest) >= 3 {
			add(rest)
			if irr, ok := irregularBases[rest]; ok {
				for _, b := range irr {
					add(b)
				}
			}
		}
	}

	// 3. Comparative & adverbial suffixes (keep polarity first)
	applySuffixRules(w, inflectionAndAdverbRules, add)

	// 4. Negation prefix `ne-`
	if strings.HasPrefix(w, "ne") && !strings.HasPrefix(w, "nej") {
		rest := strings.TrimPrefix(w, "ne")
		if utf8.RuneCountInString(rest) >= 3 {
			add(rest)
			if irr, ok := irregularBases[rest]; ok {
				for _, b := range irr {
					add(b)
				}
			}
		}
	}

	// 5. Productive derivational suffixes
	applySuffixRules(w, derivationalRules, add)

	return out
}

func applySuffixRules(w string, rules []suffixRule, add func(string)) {
	for _, r := range rules {
		if !strings.HasSuffix(w, r.suffix) {
			continue
		}
		stem := strings.TrimSuffix(w, r.suffix)
		if utf8.RuneCountInString(stem) < r.minStemRunes {
			continue
		}
		for _, repl := range r.replacements {
			cand := stem + repl
			if utf8.RuneCountInString(cand) >= 3 {
				add(cand)
			}
		}
	}
}

type lexemeCandidateRow struct {
	id       int64
	lemma    string
	priority int
}

func (s *Store) findCandidateLexemes(ctx context.Context, cand string) ([]lexemeCandidateRow, error) {
	low := textnorm.NormalizeWord(cand)
	if low == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, lemma, priority
		FROM lexemes
		WHERE lemma_lower = ?
		ORDER BY (lemma = ?) DESC, priority DESC, id ASC
		LIMIT 6
	`, low, low)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []lexemeCandidateRow
	for rows.Next() {
		var r lexemeCandidateRow
		if err := rows.Scan(&r.id, &r.lemma, &r.priority); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResolveBaseTranslations searches up to 3 morphological derivation hops to find
// the closest base lexeme in the database (and its translations for neededLangs, if any).
func (s *Store) ResolveBaseTranslations(
	ctx context.Context,
	startWord string,
	excludeLexemeID int64,
	neededLangs []string,
	stemmer StemmerFunc,
) (baseLemma string, translations []Translation, fromLemma string) {
	startNorm := textnorm.NormalizeWord(startWord)
	if startNorm == "" {
		return "", nil, ""
	}

	type node struct {
		word  string
		depth int
	}

	const maxDepth = 3
	const maxVisited = 36

	visited := map[string]bool{startNorm: true}
	queue := []node{{word: startNorm, depth: 0}}

	wantTranslations := len(neededLangs) > 0

	for len(queue) > 0 && len(visited) < maxVisited {
		cur := queue[0]
		queue = queue[1:]

		var nextWords []string
		pushCand := func(c string) {
			cn := textnorm.NormalizeWord(c)
			if cn == "" || visited[cn] {
				return
			}
			visited[cn] = true
			nextWords = append(nextWords, cn)
		}

		// 1. Morphological word_forms table (e.g. comparative/inflected -> positive lemma)
		if fms, err := s.FindFormMatches(ctx, cur.word, false); err == nil {
			for _, fm := range fms {
				if fm.LexemeID != excludeLexemeID {
					pushCand(fm.Lemma)
				}
			}
		}

		// 2. Optional Hunspell stemmer (e.g. adverb horizontálně -> horizontální, possessive ženin -> žena)
		if stemmer != nil {
			for _, st := range stemmer(cur.word) {
				pushCand(st)
			}
		}

		// 3. Czech morphological derivation rules (comparatives, adverbs, negation, deverbal/denominal)
		for _, cand := range CandidateBaseWords(cur.word) {
			pushCand(cand)
		}

		for _, cand := range nextWords {
			lexRows, err := s.findCandidateLexemes(ctx, cand)
			if err == nil {
				for _, lr := range lexRows {
					if lr.id == excludeLexemeID {
						continue
					}
					if baseLemma == "" && lr.priority > 0 {
						baseLemma = lr.lemma
					}
					if wantTranslations {
						trs, err := s.GetTranslationsByLexeme(ctx, lr.id, neededLangs)
						if err == nil && len(trs) > 0 {
							for i := range trs {
								trs[i].FromLemma = lr.lemma
							}
							if baseLemma == "" {
								baseLemma = lr.lemma
							}
							return baseLemma, trs, lr.lemma
						}
					}
				}
				if baseLemma == "" && len(lexRows) > 0 && lexRows[0].id != excludeLexemeID {
					baseLemma = lexRows[0].lemma
				}
			}

			if cur.depth+1 < maxDepth && len(visited) < maxVisited {
				queue = append(queue, node{word: cand, depth: cur.depth + 1})
			}
		}

		if !wantTranslations && baseLemma != "" {
			return baseLemma, nil, ""
		}
	}

	return baseLemma, nil, ""
}

// EnrichFallbackTranslations populates BaseLemma and borrows fallback translations from
// the closest base word when a lexeme has no translations for the requested languages.
func (s *Store) EnrichFallbackTranslations(
	ctx context.Context,
	det *LexemeDetail,
	langs []string,
	stemmer StemmerFunc,
) {
	if det == nil {
		return
	}

	var neededLangs []string
	if len(langs) > 0 && len(det.Translations) == 0 {
		neededLangs = langs
	}

	needBaseInfo := len(det.Senses) == 0 && det.BaseLemma == ""
	if len(neededLangs) == 0 && !needBaseInfo {
		return
	}

	baseLemma, fallbackTrs, fromLemma := s.ResolveBaseTranslations(ctx, det.Lemma, det.ID, neededLangs, stemmer)
	if det.BaseLemma == "" && baseLemma != "" && !strings.EqualFold(baseLemma, det.Lemma) {
		det.BaseLemma = baseLemma
	}

	if len(fallbackTrs) > 0 && len(det.Translations) == 0 {
		det.Translations = fallbackTrs
		det.TranslationsFromLemma = fromLemma
	}
}
