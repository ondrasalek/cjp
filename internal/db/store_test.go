package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreSchemaAndFTS(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	st, err := Open(dbPath, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if err := st.InitSchema(ctx); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	res, err := st.DB().ExecContext(ctx, `
		INSERT INTO lexemes (lemma, lemma_lower, lemma_ascii, pos, pos_cs, gender, ipa, hyphenation, priority, source)
		VALUES ('žena', 'žena', 'zena', 'noun', 'podstatné jméno', 'f', '[ˈʒɛ.na]', 'že-na', 2, 'cswiktionary')
	`)
	if err != nil {
		t.Fatalf("insert lexeme: %v", err)
	}
	lexID, _ := res.LastInsertId()

	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO senses (lexeme_id, sense_index, definition, source)
		VALUES (?, 1, 'dospělý člověk samičího pohlaví', 'cswiktionary')
	`, lexID)
	if err != nil {
		t.Fatalf("insert sense: %v", err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO senses_fts (lemma, definition, lexeme_id)
		VALUES ('žena', 'dospělý člověk samičího pohlaví', ?)
	`, lexID)
	if err != nil {
		t.Fatalf("insert fts: %v", err)
	}

	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO word_forms (lexeme_id, form, form_lower, form_ascii, gram_case, number, source)
		VALUES (?, 'ženami', 'ženami', 'zenami', '7', 'pl', 'cswiktionary')
	`, lexID)
	if err != nil {
		t.Fatalf("insert form: %v", err)
	}

	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO translations (lexeme_id, sense_index, lang_code, lang_name, target_word, source)
		VALUES (?, 1, 'en', 'angličtina', 'woman', 'cswiktionary'),
		       (?, 1, 'ru', 'ruština', 'женщина', 'cswiktionary')
	`, lexID, lexID)
	if err != nil {
		t.Fatalf("insert translations: %v", err)
	}

	if err := st.CreateIndexes(ctx); err != nil {
		t.Fatalf("CreateIndexes: %v", err)
	}

	// Test ASCII autocomplete ("zen" -> "žena")
	ac, err := st.Autocomplete(ctx, "zen", 5)
	if err != nil || len(ac) != 1 || ac[0].Lemma != "žena" {
		t.Fatalf("Autocomplete('zen') = %+v, err=%v", ac, err)
	}

	// Test form match ("ženami" -> case 7, pl, lemma "žena")
	fm, err := st.FindFormMatches(ctx, "ženami", true)
	if err != nil || len(fm) != 1 || fm[0].Case != "7" || fm[0].CaseName != "instrumentál" {
		t.Fatalf("FindFormMatches('ženami') = %+v, err=%v", fm, err)
	}

	// Test FTS5 search without diacritics ("samiciho pohlavi" -> "žena")
	hits, err := st.SearchDefinitions(ctx, "samiciho pohlavi", 5)
	if err != nil || len(hits) != 1 || hits[0].Lemma != "žena" {
		t.Fatalf("SearchDefinitions = %+v, err=%v", hits, err)
	}

	// Test FTS5 Czech stopword + inflection stem matching ("cloveka o samiciho" -> "žena")
	resMeta, err := st.SearchDefinitionsWithMeta(ctx, "cloveka o samiciho", 5)
	if err != nil || resMeta.Relaxed || len(resMeta.Hits) != 1 || resMeta.Hits[0].Lemma != "žena" {
		t.Fatalf("SearchDefinitionsWithMeta('cloveka o samiciho') = %+v, err=%v", resMeta, err)
	}

	// Test FTS5 relaxed BM25 fallback when not all words appear in a single definition
	relaxedMeta, err := st.SearchDefinitionsWithMeta(ctx, "neexistujicislovo pohlavi", 5)
	if err != nil || !relaxedMeta.Relaxed || len(relaxedMeta.Hits) != 1 || relaxedMeta.Hits[0].Lemma != "žena" {
		t.Fatalf("SearchDefinitionsWithMeta relaxed = %+v, err=%v", relaxedMeta, err)
	}

	// Test detail with EN/RU translations
	det, err := st.GetLexemeDetail(ctx, lexID, []string{"en", "ru"})
	if err != nil || len(det.Translations) != 2 || det.Inflection == nil {
		t.Fatalf("GetLexemeDetail = %+v, err=%v", det, err)
	}
}
