package db

const SchemaSQL = `
CREATE TABLE IF NOT EXISTS lexemes (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	lemma       TEXT NOT NULL,
	lemma_lower TEXT NOT NULL,
	lemma_ascii TEXT NOT NULL,
	pos         TEXT NOT NULL DEFAULT '',
	pos_cs      TEXT NOT NULL DEFAULT '',
	gender      TEXT NOT NULL DEFAULT '',
	animacy     TEXT NOT NULL DEFAULT '',
	aspect      TEXT NOT NULL DEFAULT '',
	ipa         TEXT NOT NULL DEFAULT '',
	audio_url   TEXT NOT NULL DEFAULT '',
	hyphenation TEXT NOT NULL DEFAULT '',
	etymology   TEXT NOT NULL DEFAULT '',
	priority    INTEGER NOT NULL DEFAULT 0,
	source      TEXT NOT NULL,
	UNIQUE(lemma, pos)
);

CREATE TABLE IF NOT EXISTS word_forms (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	lexeme_id   INTEGER NOT NULL REFERENCES lexemes(id) ON DELETE CASCADE,
	form        TEXT NOT NULL,
	form_lower  TEXT NOT NULL,
	form_ascii  TEXT NOT NULL,
	gram_case   TEXT NOT NULL DEFAULT '',
	number      TEXT NOT NULL DEFAULT '',
	person      TEXT NOT NULL DEFAULT '',
	tense       TEXT NOT NULL DEFAULT '',
	mood        TEXT NOT NULL DEFAULT '',
	voice       TEXT NOT NULL DEFAULT '',
	degree      TEXT NOT NULL DEFAULT '',
	gender      TEXT NOT NULL DEFAULT '',
	animacy     TEXT NOT NULL DEFAULT '',
	nonfinite   TEXT NOT NULL DEFAULT '',
	raw_tags    TEXT NOT NULL DEFAULT '',
	source      TEXT NOT NULL,
	UNIQUE(lexeme_id, form, gram_case, number, person, tense, mood, voice, degree, gender, animacy, nonfinite)
);

CREATE TABLE IF NOT EXISTS senses (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	lexeme_id     INTEGER NOT NULL REFERENCES lexemes(id) ON DELETE CASCADE,
	sense_index   INTEGER NOT NULL,
	definition    TEXT NOT NULL,
	labels        TEXT NOT NULL DEFAULT '',
	examples_json TEXT NOT NULL DEFAULT '[]',
	source        TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS senses_fts USING fts5(
	lemma,
	definition,
	lexeme_id UNINDEXED,
	tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TABLE IF NOT EXISTS relations (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	lexeme_id   INTEGER NOT NULL REFERENCES lexemes(id) ON DELETE CASCADE,
	sense_index INTEGER NOT NULL DEFAULT 0,
	rel_type    TEXT NOT NULL,
	target_word TEXT NOT NULL,
	qualifier   TEXT NOT NULL DEFAULT '',
	source      TEXT NOT NULL,
	UNIQUE(lexeme_id, sense_index, rel_type, target_word)
);

CREATE TABLE IF NOT EXISTS translations (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	lexeme_id   INTEGER NOT NULL REFERENCES lexemes(id) ON DELETE CASCADE,
	sense_index INTEGER NOT NULL DEFAULT 0,
	sense_gloss TEXT NOT NULL DEFAULT '',
	lang_code   TEXT NOT NULL,
	lang_name   TEXT NOT NULL DEFAULT '',
	target_word TEXT NOT NULL,
	qualifier   TEXT NOT NULL DEFAULT '',
	source      TEXT NOT NULL,
	UNIQUE(lexeme_id, sense_index, lang_code, target_word)
);

CREATE TABLE IF NOT EXISTS data_sources (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	license      TEXT NOT NULL,
	url          TEXT NOT NULL,
	attribution  TEXT NOT NULL,
	record_count INTEGER NOT NULL DEFAULT 0,
	imported_at  TEXT NOT NULL
);
`

const IndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_lexemes_lower ON lexemes(lemma_lower, priority DESC);
CREATE INDEX IF NOT EXISTS idx_lexemes_ascii ON lexemes(lemma_ascii, priority DESC);
CREATE INDEX IF NOT EXISTS idx_forms_lower ON word_forms(form_lower);
CREATE INDEX IF NOT EXISTS idx_forms_ascii ON word_forms(form_ascii);
CREATE INDEX IF NOT EXISTS idx_forms_lexeme ON word_forms(lexeme_id);
CREATE INDEX IF NOT EXISTS idx_senses_lexeme ON senses(lexeme_id, sense_index);
CREATE INDEX IF NOT EXISTS idx_relations_lexeme ON relations(lexeme_id, rel_type);
CREATE INDEX IF NOT EXISTS idx_translations_lexeme ON translations(lexeme_id, lang_code);
`
