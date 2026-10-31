package etl

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ondrejsalek/cjp/internal/db"
	"github.com/ondrejsalek/cjp/internal/hunspell"
	"github.com/ondrejsalek/cjp/internal/hyphen"
	"github.com/ondrejsalek/cjp/internal/textnorm"
)

// BuildOptions configures the ETL build pipeline.
type BuildOptions struct {
	RawDir     string
	OutDBPath  string
	IncludeGPL bool
}

type lexemeKey struct {
	lemma string
	pos   string
}

type lexemeMeta struct {
	id         int64
	senseCount int
}

type wiktEntry struct {
	Word           string            `json:"word"`
	LangCode       string            `json:"lang_code"`
	POS            string            `json:"pos"`
	POSTitle       string            `json:"pos_title"`
	Tags           []string          `json:"tags"`
	RawTags        []string          `json:"raw_tags"`
	EtymologyTexts []string          `json:"etymology_texts"`
	Sounds         []wiktSound       `json:"sounds"`
	Hyphenations   []wiktHyphenation `json:"hyphenations"`
	Senses         []wiktSense       `json:"senses"`
	Forms          []wiktForm        `json:"forms"`
	Synonyms       []wiktRel         `json:"synonyms"`
	Antonyms       []wiktRel         `json:"antonyms"`
	Related        []wiktRel         `json:"related"`
	Phrases        []wiktRel         `json:"phrases"`
	Proverbs       []wiktRel         `json:"proverbs"`
	Translations   []wiktTranslation `json:"translations"`
}

type wiktSound struct {
	IPA    string `json:"ipa"`
	MP3URL string `json:"mp3_url"`
	OggURL string `json:"ogg_url"`
}

type wiktHyphenation struct {
	Parts []string `json:"parts"`
}

type wiktSense struct {
	Glosses  []string      `json:"glosses"`
	Tags     []string      `json:"tags"`
	RawTags  []string      `json:"raw_tags"`
	Topics   []string      `json:"topics"`
	Examples []wiktExample `json:"examples"`
	FormOf   []wiktFormRef `json:"form_of"`
}

type wiktExample struct {
	Text string `json:"text"`
	Ref  string `json:"ref"`
}

type wiktFormRef struct {
	Word string `json:"word"`
}

type wiktForm struct {
	Form    string   `json:"form"`
	Tags    []string `json:"tags"`
	RawTags []string `json:"raw_tags"`
}

type wiktRel struct {
	Word       string   `json:"word"`
	SenseIndex int      `json:"sense_index"`
	Tags       []string `json:"tags"`
	RawTags    []string `json:"raw_tags"`
}

type wiktTranslation struct {
	LangCode   string   `json:"lang_code"`
	Lang       string   `json:"lang"`
	Word       string   `json:"word"`
	Sense      string   `json:"sense"`
	SenseIndex int      `json:"sense_index"`
	Tags       []string `json:"tags"`
	RawTags    []string `json:"raw_tags"`
}

type deferredFormStub struct {
	form   string
	pos    string
	target string
	gloss  string
}

// BuildDatabase builds a fresh SQLite lexical database from downloaded open datasets.
func BuildDatabase(ctx context.Context, opts BuildOptions) error {
	start := time.Now()
	if err := os.MkdirAll(filepath.Dir(opts.OutDBPath), 0o755); err != nil {
		return err
	}

	tmpDB := opts.OutDBPath + ".building"
	_ = os.Remove(tmpDB)
	_ = os.Remove(tmpDB + "-wal")
	_ = os.Remove(tmpDB + "-shm")

	store, err := db.Open(tmpDB, false)
	if err != nil {
		return err
	}
	defer func() {
		_ = store.Close()
		_ = os.Remove(tmpDB)
		_ = os.Remove(tmpDB + "-wal")
		_ = os.Remove(tmpDB + "-shm")
	}()

	// Fast bulk load pragmas while building tmpDB
	_, _ = store.DB().ExecContext(ctx, `PRAGMA synchronous = OFF; PRAGMA journal_mode = MEMORY; PRAGMA temp_store = MEMORY;`)

	if err := store.InitSchema(ctx); err != nil {
		return err
	}

	hyphPath := filepath.Join(opts.RawDir, "hyph_cs_CZ.dic")
	hyph, err := hyphen.LoadFile(hyphPath)
	if err != nil {
		return fmt.Errorf("load hyphenation patterns (%s): %w", hyphPath, err)
	}

	lexMap := make(map[lexemeKey]*lexemeMeta, 320000)
	byLemma := make(map[string][]int64, 300000)

	// Stage 1: Czech Wiktionary (Kaikki JSONL)
	wiktPath := filepath.Join(opts.RawDir, "cs-extract.jsonl.gz")
	fmt.Printf("[1/4] Importing Czech Wiktionary (%s)...\n", wiktPath)
	wiktCount, err := importWiktionary(ctx, store.DB(), wiktPath, hyph, lexMap, byLemma)
	if err != nil {
		return fmt.Errorf("import wiktionary: %w", err)
	}
	if err := recordSource(ctx, store.DB(), db.DataSourceInfo{
		ID:          "cswiktionary",
		Name:        "Český Wikislovník (Kaikki.org JSONL extract)",
		License:     "CC BY-SA 4.0",
		URL:         "https://cs.wiktionary.org / https://kaikki.org/dictionary/downloads/cs/",
		Attribution: "Obsah pochází z Českého Wikislovníku (cs.wiktionary.org), zpracováno nástrojem Wiktextract (kaikki.org), šířeno pod licencí CC BY-SA 4.0.",
		RecordCount: wiktCount,
		ImportedAt:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	fmt.Printf("      -> imported %d Czech Wiktionary headword entries (%d unique lexemes)\n", wiktCount, len(lexMap))

	// Stage 2: UniMorph Czech (Wiktionary branch, CC BY-SA 3.0)
	uniPath := filepath.Join(opts.RawDir, "unimorph_ces.tsv")
	fmt.Printf("[2/4] Importing UniMorph Czech (%s)...\n", uniPath)
	uniCount, err := importUniMorph(ctx, store.DB(), uniPath, hyph, lexMap, byLemma)
	if err != nil {
		return fmt.Errorf("import unimorph: %w", err)
	}
	if err := recordSource(ctx, store.DB(), db.DataSourceInfo{
		ID:          "unimorph",
		Name:        "UniMorph Czech (Wiktionary morphological paradigms)",
		License:     "CC BY-SA 3.0",
		URL:         "https://github.com/unimorph/ces",
		Attribution: "Morfologické tabulky projektu UniMorph (unimorph/ces, větev Wiktionary), šířeno pod licencí CC BY-SA 3.0.",
		RecordCount: uniCount,
		ImportedAt:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	fmt.Printf("      -> imported %d UniMorph morphological forms\n", uniCount)

	// Record hyphenation pattern metadata
	_ = recordSource(ctx, store.DB(), db.DataSourceInfo{
		ID:          "hyph_cs",
		Name:        "České vzory dělení slov (csTeX / LibreOffice)",
		License:     "GPL (externí vzorový soubor)",
		URL:         "https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ",
		Attribution: "Vzory českého dělení slov (Pavel Janík, csTeX / LibreOffice).",
		RecordCount: 1,
		ImportedAt:  time.Now().UTC().Format(time.RFC3339),
	})

	// Stage 3: Optional GPL datasets (Thesaurus + Hunspell stems)
	if opts.IncludeGPL {
		thesPath := filepath.Join(opts.RawDir, "th_cs_CZ.dat")
		if _, err := os.Stat(thesPath); err == nil {
			fmt.Printf("[3/4] Importing Czech Thesaurus (%s)...\n", thesPath)
			thesCount, err := importThesaurus(ctx, store.DB(), thesPath, hyph, lexMap, byLemma)
			if err != nil {
				return fmt.Errorf("import thesaurus: %w", err)
			}
			_ = recordSource(ctx, store.DB(), db.DataSourceInfo{
				ID:          "thesaurus_cs",
				Name:        "Český tezaurus (LibreOffice cs_CZ)",
				License:     "GFDL / GPL",
				URL:         "https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ",
				Attribution: "Český slovník synonym z balíku LibreOffice cs_CZ.",
				RecordCount: thesCount,
				ImportedAt:  time.Now().UTC().Format(time.RFC3339),
			})
			fmt.Printf("      -> imported %d thesaurus synonym relations\n", thesCount)
		}

		affPath := filepath.Join(opts.RawDir, "cs_CZ.aff")
		dicPath := filepath.Join(opts.RawDir, "cs_CZ.dic")
		if _, err := os.Stat(dicPath); err == nil {
			fmt.Printf("[3b/4] Importing Czech Hunspell vocabulary stems (%s)...\n", dicPath)
			hunCount, err := importHunspellStems(ctx, store.DB(), affPath, dicPath, hyph, lexMap, byLemma)
			if err != nil {
				return fmt.Errorf("import hunspell: %w", err)
			}
			_ = recordSource(ctx, store.DB(), db.DataSourceInfo{
				ID:          "hunspell_cs",
				Name:        "Český pravopisný slovník Hunspell (cs_CZ)",
				License:     "GPL",
				URL:         "https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ",
				Attribution: "Český slovník pro kontrolu pravopisu (Petr Kolář, Miroslav Pošta a přispěvatelé, LibreOffice cs_CZ).",
				RecordCount: hunCount,
				ImportedAt:  time.Now().UTC().Format(time.RFC3339),
			})
			fmt.Printf("      -> added %d additional Hunspell headword stems (total lexemes: %d)\n", hunCount, len(lexMap))
		}
	} else {
		fmt.Printf("[3/4] Skipping GPL datasets (--include-gpl=false)\n")
	}

	// Stage 4: Build indexes & finalize SQLite file
	fmt.Printf("[4/4] Building B-tree & FTS5 indexes...\n")
	if err := store.CreateIndexes(ctx); err != nil {
		return err
	}
	_, _ = store.DB().ExecContext(ctx, `PRAGMA journal_mode = DELETE; PRAGMA optimize;`)
	if err := store.Close(); err != nil {
		return err
	}

	_ = os.Remove(opts.OutDBPath + "-wal")
	_ = os.Remove(opts.OutDBPath + "-shm")
	if err := os.Rename(tmpDB, opts.OutDBPath); err != nil {
		return fmt.Errorf("finalize db rename: %w", err)
	}

	fi, _ := os.Stat(opts.OutDBPath)
	sizeMB := float64(0)
	if fi != nil {
		sizeMB = float64(fi.Size()) / (1024 * 1024)
	}
	fmt.Printf("[done] Built %s (%.1f MB) in %s\n", opts.OutDBPath, sizeMB, time.Since(start).Round(time.Millisecond))
	return nil
}

func importWiktionary(
	ctx context.Context,
	sqldb *sql.DB,
	gzPath string,
	hyph *hyphen.Hyphenator,
	lexMap map[lexemeKey]*lexemeMeta,
	byLemma map[string][]int64,
) (int64, error) {
	f, err := os.Open(gzPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gz.Close()

	tx, err := sqldb.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insLex, err := tx.PrepareContext(ctx, `
		INSERT INTO lexemes (lemma, lemma_lower, lemma_ascii, pos, pos_cs, gender, animacy, aspect, ipa, audio_url, hyphenation, etymology, priority, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 2, 'cswiktionary')
	`)
	if err != nil {
		return 0, err
	}
	defer insLex.Close()

	updLex, err := tx.PrepareContext(ctx, `
		UPDATE lexemes
		SET ipa = CASE WHEN ipa = '' THEN ? ELSE ipa END,
		    audio_url = CASE WHEN audio_url = '' THEN ? ELSE audio_url END,
		    hyphenation = CASE WHEN hyphenation = '' THEN ? ELSE hyphenation END,
		    etymology = CASE WHEN etymology = '' THEN ? ELSE etymology END
		WHERE id = ?
	`)
	if err != nil {
		return 0, err
	}
	defer updLex.Close()

	insSense, err := tx.PrepareContext(ctx, `
		INSERT INTO senses (lexeme_id, sense_index, definition, labels, examples_json, source)
		VALUES (?, ?, ?, ?, ?, 'cswiktionary')
	`)
	if err != nil {
		return 0, err
	}
	defer insSense.Close()

	insFTS, err := tx.PrepareContext(ctx, `
		INSERT INTO senses_fts (lemma, definition, lexeme_id) VALUES (?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer insFTS.Close()

	insForm, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO word_forms (
			lexeme_id, form, form_lower, form_ascii,
			gram_case, number, person, tense, mood, voice, degree, gender, animacy, nonfinite, raw_tags, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'cswiktionary')
	`)
	if err != nil {
		return 0, err
	}
	defer insForm.Close()

	insRel, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO relations (lexeme_id, sense_index, rel_type, target_word, qualifier, source)
		VALUES (?, ?, ?, ?, ?, 'cswiktionary')
	`)
	if err != nil {
		return 0, err
	}
	defer insRel.Close()

	insTrans, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO translations (lexeme_id, sense_index, sense_gloss, lang_code, lang_name, target_word, qualifier, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'cswiktionary')
	`)
	if err != nil {
		return 0, err
	}
	defer insTrans.Close()

	var stubs []deferredFormStub
	var headwords int64

	reader := bufio.NewReaderSize(gz, 512*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Fast pre-filter before full JSON unmarshal
			if !strings.Contains(string(line), `"lang_code": "cs"`) && !strings.Contains(string(line), `"lang_code":"cs"`) {
				if readErr != nil {
					if errors.Is(readErr, io.EOF) {
						break
					}
					return 0, readErr
				}
				continue
			}

			var e wiktEntry
			if err := json.Unmarshal(line, &e); err == nil && e.LangCode == "cs" && strings.TrimSpace(e.Word) != "" {
				lemma := strings.TrimSpace(e.Word)
				pos, posCs := NormalizePOS(e.POS, e.POSTitle)

				if isFormOfEntry(&e) {
					for _, s := range e.Senses {
						gloss := strings.Join(s.Glosses, "; ")
						for _, fo := range s.FormOf {
							if strings.TrimSpace(fo.Word) != "" {
								stubs = append(stubs, deferredFormStub{
									form:   lemma,
									pos:    pos,
									target: strings.TrimSpace(fo.Word),
									gloss:  gloss,
								})
							}
						}
					}
				} else {
					headwords++
					ipa, audio := extractSound(e.Sounds)
					hyphStr := extractHyphenation(e.Hyphenations, lemma, hyph)
					etym := strings.TrimSpace(strings.Join(e.EtymologyTexts, " "))
					gender, animacy, aspect := ParseLexemeTags(e.Tags, e.RawTags)

					key := lexemeKey{lemma: lemma, pos: pos}
					meta, exists := lexMap[key]
					if !exists {
						res, err := insLex.ExecContext(ctx,
							lemma,
							textnorm.NormalizeWord(lemma),
							textnorm.StripDiacritics(lemma),
							pos, posCs, gender, animacy, aspect,
							ipa, audio, hyphStr, etym,
						)
						if err != nil {
							return 0, err
						}
						id, _ := res.LastInsertId()
						meta = &lexemeMeta{id: id}
						lexMap[key] = meta
						byLemma[lemma] = append(byLemma[lemma], id)
					} else {
						_, _ = updLex.ExecContext(ctx, ipa, audio, hyphStr, etym, meta.id)
					}

					senseOffset := meta.senseCount
					for i, s := range e.Senses {
						if len(s.FormOf) > 0 {
							for _, fo := range s.FormOf {
								if strings.TrimSpace(fo.Word) != "" {
									stubs = append(stubs, deferredFormStub{
										form:   lemma,
										pos:    pos,
										target: strings.TrimSpace(fo.Word),
										gloss:  strings.Join(s.Glosses, "; "),
									})
								}
							}
							continue
						}
						def := strings.TrimSpace(strings.Join(s.Glosses, "; "))
						if def == "" {
							continue
						}
						meta.senseCount++
						sIdx := senseOffset + i + 1

						var labels []string
						for _, t := range s.Tags {
							if t != "form-of" {
								labels = append(labels, TranslateLabelCs(t))
							}
						}
						labels = append(labels, s.RawTags...)
						labels = append(labels, s.Topics...)

						exBytes := []byte("[]")
						if len(s.Examples) > 0 {
							var exs []db.ExampleSentence
							for _, ex := range s.Examples {
								if strings.TrimSpace(ex.Text) != "" {
									exs = append(exs, db.ExampleSentence{
										Text: strings.TrimSpace(ex.Text),
										Ref:  strings.TrimSpace(ex.Ref),
									})
								}
							}
							if len(exs) > 0 {
								exBytes, _ = json.Marshal(exs)
							}
						}

						if _, err := insSense.ExecContext(ctx, meta.id, sIdx, def, strings.Join(labels, ","), string(exBytes)); err != nil {
							return 0, err
						}
						if _, err := insFTS.ExecContext(ctx, lemma, def, meta.id); err != nil {
							return 0, err
						}
					}

					// Base canonical form
					baseCase, baseNum, baseNonFin := "", "", ""
					switch pos {
					case "noun", "adj", "pron", "num":
						baseCase, baseNum = "1", "sg"
					case "verb":
						baseNonFin = "inf"
					}
					_, _ = insForm.ExecContext(ctx,
						meta.id, lemma, textnorm.NormalizeWord(lemma), textnorm.StripDiacritics(lemma),
						baseCase, baseNum, "", "", "", "", "", gender, animacy, baseNonFin, "",
					)

					// Inflected forms
					for _, fm := range e.Forms {
						fw := strings.TrimSpace(fm.Form)
						if !isValidFormWord(fw) {
							continue
						}
						pm, ok := ParseWiktionaryFormTags(fm.Tags)
						if !ok {
							continue
						}
						extra := append(pm.ExtraTags, fm.RawTags...)
						_, _ = insForm.ExecContext(ctx,
							meta.id, fw, textnorm.NormalizeWord(fw), textnorm.StripDiacritics(fw),
							pm.Case, pm.Number, pm.Person, pm.Tense, pm.Mood, pm.Voice,
							pm.Degree, pm.Gender, pm.Animacy, pm.NonFinite, strings.Join(extra, ","),
						)
					}

					// Lexical relations
					insertRels := func(relType string, items []wiktRel) {
						for _, r := range items {
							tw := strings.TrimSpace(r.Word)
							if !isValidFormWord(tw) {
								continue
							}
							var quals []string
							for _, t := range r.Tags {
								quals = append(quals, TranslateLabelCs(t))
							}
							quals = append(quals, r.RawTags...)
							sIdx := r.SenseIndex
							if sIdx > 0 && senseOffset > 0 {
								sIdx += senseOffset
							}
							_, _ = insRel.ExecContext(ctx, meta.id, sIdx, relType, tw, strings.Join(quals, ","))
						}
					}
					insertRels("synonym", e.Synonyms)
					insertRels("antonym", e.Antonyms)
					insertRels("related", e.Related)
					insertRels("phrase", e.Phrases)
					insertRels("proverb", e.Proverbs)

					// Translations (EN, RU, DE, SK, etc.)
					for _, tr := range e.Translations {
						lc := strings.ToLower(strings.TrimSpace(tr.LangCode))
						tw := strings.TrimSpace(tr.Word)
						if lc == "" || !isValidFormWord(tw) {
							continue
						}
						var quals []string
						for _, t := range tr.Tags {
							quals = append(quals, TranslateLabelCs(t))
						}
						quals = append(quals, tr.RawTags...)
						sIdx := tr.SenseIndex
						if sIdx > 0 && senseOffset > 0 {
							sIdx += senseOffset
						}
						_, _ = insTrans.ExecContext(ctx,
							meta.id, sIdx, strings.TrimSpace(tr.Sense),
							lc, strings.TrimSpace(tr.Lang), tw, strings.Join(quals, ","),
						)
					}
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return 0, readErr
		}
	}

	// Link deferred form-of stubs to their target lemmas
	for _, st := range stubs {
		var targetID int64
		if m, ok := lexMap[lexemeKey{lemma: st.target, pos: st.pos}]; ok {
			targetID = m.id
		} else if ids := byLemma[st.target]; len(ids) > 0 {
			targetID = ids[0]
		}
		if targetID == 0 {
			continue
		}
		pm := parseCzechGlossMorph(st.gloss)
		_, _ = insForm.ExecContext(ctx,
			targetID, st.form, textnorm.NormalizeWord(st.form), textnorm.StripDiacritics(st.form),
			pm.Case, pm.Number, pm.Person, pm.Tense, pm.Mood, pm.Voice,
			pm.Degree, pm.Gender, pm.Animacy, pm.NonFinite, "",
		)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return headwords, nil
}

func importUniMorph(
	ctx context.Context,
	sqldb *sql.DB,
	tsvPath string,
	hyph *hyphen.Hyphenator,
	lexMap map[lexemeKey]*lexemeMeta,
	byLemma map[string][]int64,
) (int64, error) {
	f, err := os.Open(tsvPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	tx, err := sqldb.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insLex, err := tx.PrepareContext(ctx, `
		INSERT INTO lexemes (lemma, lemma_lower, lemma_ascii, pos, pos_cs, hyphenation, priority, source)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'unimorph')
	`)
	if err != nil {
		return 0, err
	}
	defer insLex.Close()

	insForm, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO word_forms (
			lexeme_id, form, form_lower, form_ascii,
			gram_case, number, person, tense, mood, voice, degree, gender, animacy, nonfinite, raw_tags, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unimorph')
	`)
	if err != nil {
		return 0, err
	}
	defer insForm.Close()

	var count int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		lemma := strings.TrimSpace(parts[0])
		form := strings.TrimSpace(parts[1])
		if lemma == "" || !isValidFormWord(form) {
			continue
		}
		pos, posCs, pm := ParseUniMorphTags(parts[2])
		if pos == "" {
			continue
		}

		key := lexemeKey{lemma: lemma, pos: pos}
		meta, exists := lexMap[key]
		if !exists {
			hyphStr := hyph.Hyphenate(lemma, "-")
			res, err := insLex.ExecContext(ctx,
				lemma, textnorm.NormalizeWord(lemma), textnorm.StripDiacritics(lemma),
				pos, posCs, hyphStr,
			)
			if err != nil {
				return 0, err
			}
			id, _ := res.LastInsertId()
			meta = &lexemeMeta{id: id}
			lexMap[key] = meta
			byLemma[lemma] = append(byLemma[lemma], id)
		}

		_, err = insForm.ExecContext(ctx,
			meta.id, form, textnorm.NormalizeWord(form), textnorm.StripDiacritics(form),
			pm.Case, pm.Number, pm.Person, pm.Tense, pm.Mood, pm.Voice,
			pm.Degree, pm.Gender, pm.Animacy, pm.NonFinite, strings.Join(pm.ExtraTags, ","),
		)
		if err != nil {
			return 0, err
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func importThesaurus(
	ctx context.Context,
	sqldb *sql.DB,
	datPath string,
	hyph *hyphen.Hyphenator,
	lexMap map[lexemeKey]*lexemeMeta,
	byLemma map[string][]int64,
) (int64, error) {
	f, err := os.Open(datPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	tx, err := sqldb.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insLex, err := tx.PrepareContext(ctx, `
		INSERT INTO lexemes (lemma, lemma_lower, lemma_ascii, pos, pos_cs, hyphenation, priority, source)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'thesaurus_cs')
	`)
	if err != nil {
		return 0, err
	}
	defer insLex.Close()

	insRel, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO relations (lexeme_id, sense_index, rel_type, target_word, qualifier, source)
		VALUES (?, ?, ?, ?, ?, 'thesaurus_cs')
	`)
	if err != nil {
		return 0, err
	}
	defer insRel.Close()

	scanner := bufio.NewScanner(f)
	// Skip encoding header line ("UTF-8")
	if scanner.Scan() {
		first := strings.TrimSpace(scanner.Text())
		if strings.Contains(first, "|") {
			// Unexpected: header was already an entry; handled cleanly below if needed
		}
	}

	var relCount int64
	for scanner.Scan() {
		header := strings.TrimSpace(scanner.Text())
		if header == "" {
			continue
		}
		sep := strings.LastIndexByte(header, '|')
		if sep <= 0 {
			continue
		}
		headword := strings.TrimSpace(header[:sep])
		numLines, err := strconv.Atoi(strings.TrimSpace(header[sep+1:]))
		if err != nil || numLines <= 0 {
			continue
		}

		for i := 0; i < numLines && scanner.Scan(); i++ {
			row := strings.TrimSpace(scanner.Text())
			cols := strings.Split(row, "|")
			if len(cols) < 2 {
				continue
			}
			posHint, posCsHint := parseThesaurusPOS(cols[0])
			lexID := resolveOrCreateThesaurusLexeme(ctx, insLex, headword, posHint, posCsHint, hyph, lexMap, byLemma)
			if lexID == 0 {
				continue
			}

			for _, rawSyn := range cols[1:] {
				word, relType, qual := parseThesaurusTarget(rawSyn)
				if word == "" || word == headword {
					continue
				}
				if _, err := insRel.ExecContext(ctx, lexID, i+1, relType, word, qual); err == nil {
					relCount++
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return relCount, nil
}

func importHunspellStems(
	ctx context.Context,
	sqldb *sql.DB,
	affPath, dicPath string,
	hyph *hyphen.Hyphenator,
	lexMap map[lexemeKey]*lexemeMeta,
	byLemma map[string][]int64,
) (int64, error) {
	hd, err := hunspell.Load(affPath, dicPath)
	if err != nil {
		return 0, err
	}

	tx, err := sqldb.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insLex, err := tx.PrepareContext(ctx, `
		INSERT INTO lexemes (lemma, lemma_lower, lemma_ascii, pos, pos_cs, hyphenation, priority, source)
		VALUES (?, ?, ?, '', '', ?, 0, 'hunspell_cs')
	`)
	if err != nil {
		return 0, err
	}
	defer insLex.Close()

	var added int64
	err = hd.ForEachStem(func(stem string, _ []string) error {
		if len(byLemma[stem]) > 0 {
			return nil
		}
		hyphStr := hyph.Hyphenate(stem, "-")
		res, err := insLex.ExecContext(ctx,
			stem, textnorm.NormalizeWord(stem), textnorm.StripDiacritics(stem), hyphStr,
		)
		if err != nil {
			return nil // ignore duplicate if any
		}
		id, _ := res.LastInsertId()
		key := lexemeKey{lemma: stem, pos: ""}
		lexMap[key] = &lexemeMeta{id: id}
		byLemma[stem] = append(byLemma[stem], id)
		added++
		return nil
	})
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}

func recordSource(ctx context.Context, sqldb *sql.DB, ds db.DataSourceInfo) error {
	_, err := sqldb.ExecContext(ctx, `
		INSERT OR REPLACE INTO data_sources (id, name, license, url, attribution, record_count, imported_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ds.ID, ds.Name, ds.License, ds.URL, ds.Attribution, ds.RecordCount, ds.ImportedAt)
	return err
}

func isFormOfEntry(e *wiktEntry) bool {
	for _, t := range e.Tags {
		if t == "form-of" {
			return true
		}
	}
	if len(e.Senses) == 0 {
		return false
	}
	for _, s := range e.Senses {
		hasFormOf := len(s.FormOf) > 0
		for _, t := range s.Tags {
			if t == "form-of" {
				hasFormOf = true
				break
			}
		}
		if !hasFormOf {
			return false
		}
	}
	return true
}

func extractSound(sounds []wiktSound) (ipa, audioURL string) {
	for _, s := range sounds {
		if ipa == "" && strings.TrimSpace(s.IPA) != "" {
			ipa = strings.TrimSpace(s.IPA)
		}
		if audioURL == "" {
			if s.MP3URL != "" {
				audioURL = s.MP3URL
			} else if s.OggURL != "" {
				audioURL = s.OggURL
			}
		}
	}
	return ipa, audioURL
}

func extractHyphenation(hyphs []wiktHyphenation, lemma string, fallback *hyphen.Hyphenator) string {
	for _, h := range hyphs {
		if len(h.Parts) > 0 {
			var clean []string
			validSyllables := true
			for _, p := range h.Parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if !isValidCzechSyllable(p) {
					validSyllables = false
					break
				}
				clean = append(clean, p)
			}
			if validSyllables && len(clean) > 0 {
				return strings.Join(clean, "-")
			}
		}
	}
	return fallback.Hyphenate(lemma, "-")
}

func isValidCzechSyllable(part string) bool {
	const vowels = "aeiouyáéíóúůýěäöüAEIOUYÁÉÍÓÚŮÝĚÄÖÜ"
	if strings.ContainsAny(part, vowels) {
		return true
	}
	// Czech syllabic consonants (r, l, m) require at least one adjacent consonant (e.g. "vr", "vl", "krk", "osm")
	if len([]rune(part)) >= 2 && strings.ContainsAny(part, "rlmRLM") {
		return true
	}
	return false
}

func isValidFormWord(w string) bool {
	if w == "" || w == "-" || w == "–" || w == "—" || w == "?" {
		return false
	}
	return true
}

func parseCzechGlossMorph(gloss string) ParsedMorph {
	low := strings.ToLower(gloss)
	var m ParsedMorph
	switch {
	case strings.Contains(low, "nominativ"):
		m.Case = "1"
	case strings.Contains(low, "genitiv"):
		m.Case = "2"
	case strings.Contains(low, "dativ"):
		m.Case = "3"
	case strings.Contains(low, "akuzativ"):
		m.Case = "4"
	case strings.Contains(low, "vokativ"):
		m.Case = "5"
	case strings.Contains(low, "lokál"):
		m.Case = "6"
	case strings.Contains(low, "instrumentál"):
		m.Case = "7"
	}
	switch {
	case strings.Contains(low, "singulár") || strings.Contains(low, "jednotného"):
		m.Number = "sg"
	case strings.Contains(low, "plurál") || strings.Contains(low, "množného"):
		m.Number = "pl"
	case strings.Contains(low, "duál"):
		m.Number = "du"
	}
	switch {
	case strings.Contains(low, "1. osob") || strings.Contains(low, "první osob"):
		m.Person = "1"
	case strings.Contains(low, "2. osob") || strings.Contains(low, "druhé osob"):
		m.Person = "2"
	case strings.Contains(low, "3. osob") || strings.Contains(low, "třetí osob"):
		m.Person = "3"
	}
	switch {
	case strings.Contains(low, "rozkazovací") || strings.Contains(low, "imperativ"):
		m.Mood = "imp"
	case strings.Contains(low, "přítomného") || strings.Contains(low, "prézent"):
		m.Tense = "pres"
	case strings.Contains(low, "budoucího") || strings.Contains(low, "futur"):
		m.Tense = "fut"
	}
	if strings.Contains(low, "komparativ") || strings.Contains(low, "2. stup") {
		m.Degree = "2"
	} else if strings.Contains(low, "superlativ") || strings.Contains(low, "3. stup") {
		m.Degree = "3"
	}
	return m
}

func parseThesaurusPOS(raw string) (string, string) {
	switch strings.TrimSpace(raw) {
	case "(podst. jm.)":
		return "noun", "podstatné jméno"
	case "(slov.)":
		return "verb", "sloveso"
	case "(příd. jm.)":
		return "adj", "přídavné jméno"
	case "(přísl.)":
		return "adv", "příslovce"
	default:
		return "", ""
	}
}

func resolveOrCreateThesaurusLexeme(
	ctx context.Context,
	insLex *sql.Stmt,
	headword, posHint, posCsHint string,
	hyph *hyphen.Hyphenator,
	lexMap map[lexemeKey]*lexemeMeta,
	byLemma map[string][]int64,
) int64 {
	if posHint != "" {
		if m, ok := lexMap[lexemeKey{lemma: headword, pos: posHint}]; ok {
			return m.id
		}
	}
	if ids := byLemma[headword]; len(ids) > 0 {
		return ids[0]
	}
	hyphStr := hyph.Hyphenate(headword, "-")
	res, err := insLex.ExecContext(ctx,
		headword, textnorm.NormalizeWord(headword), textnorm.StripDiacritics(headword),
		posHint, posCsHint, hyphStr,
	)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	lexMap[lexemeKey{lemma: headword, pos: posHint}] = &lexemeMeta{id: id}
	byLemma[headword] = append(byLemma[headword], id)
	return id
}

func parseThesaurusTarget(raw string) (word, relType, qualifier string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", ""
	}
	relType = "synonym"
	if idx := strings.IndexByte(s, '('); idx > 0 && strings.HasSuffix(s, ")") {
		qual := strings.TrimSpace(s[idx+1 : len(s)-1])
		s = strings.TrimSpace(s[:idx])
		if strings.Contains(strings.ToLower(qual), "antonym") {
			relType = "antonym"
		} else {
			qualifier = qual
		}
	}
	return s, relType, qualifier
}
