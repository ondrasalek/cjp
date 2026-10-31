package db

// ExampleSentence represents an example usage of a word sense.
type ExampleSentence struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
}

// Sense represents a single meaning (definition) of a lexeme.
type Sense struct {
	SenseIndex int               `json:"sense_index"`
	Definition string            `json:"definition"`
	Labels     []string          `json:"labels,omitempty"`
	Examples   []ExampleSentence `json:"examples,omitempty"`
	Source     string            `json:"source"`
}

// Relation represents a lexical relation (synonym, antonym, related term, etc.).
type Relation struct {
	SenseIndex int    `json:"sense_index,omitempty"`
	Type       string `json:"type"`
	Word       string `json:"word"`
	Qualifier  string `json:"qualifier,omitempty"`
	Source     string `json:"source"`
}

// Translation represents a translation of a Czech lexeme into another language.
type Translation struct {
	SenseIndex int    `json:"sense_index,omitempty"`
	SenseGloss string `json:"sense_gloss,omitempty"`
	LangCode   string `json:"lang_code"`
	LangName   string `json:"lang_name,omitempty"`
	Word       string `json:"word"`
	Qualifier  string `json:"qualifier,omitempty"`
	FromLemma  string `json:"from_lemma,omitempty"`
	Source     string `json:"source"`
}

// WordForm represents a single inflected form of a lexeme.
type WordForm struct {
	Form      string   `json:"form"`
	Case      string   `json:"case,omitempty"`      // "1".."7"
	CaseName  string   `json:"case_name,omitempty"` // "nominativ", "genitiv", ...
	Number    string   `json:"number,omitempty"`    // "sg", "pl", "du"
	Person    string   `json:"person,omitempty"`    // "1", "2", "3"
	Tense     string   `json:"tense,omitempty"`     // "pres", "past", "fut"
	Mood      string   `json:"mood,omitempty"`      // "ind", "imp", "cond"
	Voice     string   `json:"voice,omitempty"`     // "act", "pass"
	Degree    string   `json:"degree,omitempty"`    // "1", "2", "3"
	Gender    string   `json:"gender,omitempty"`    // "m", "f", "n"
	Animacy   string   `json:"animacy,omitempty"`   // "anim", "inan"
	NonFinite string   `json:"nonfinite,omitempty"` // "inf", "part", "transgr"
	Tags      []string `json:"tags,omitempty"`
	Source    string   `json:"source"`
}

// CaseRow represents one row (1st..7th grammatical case) in a Czech declension table.
type CaseRow struct {
	Case     string   `json:"case"`
	CaseName string   `json:"case_name"`
	Singular []string `json:"singular,omitempty"`
	Plural   []string `json:"plural,omitempty"`
}

// ConjugationTable organizes Czech verb forms cleanly like a grammar reference.
type ConjugationTable struct {
	Infinitive      []string            `json:"infinitive,omitempty"`
	PresentOrFuture map[string][]string `json:"present_or_future,omitempty"` // "1.sg", "2.sg", "3.sg", "1.pl", "2.pl", "3.pl"
	Imperative      map[string][]string `json:"imperative,omitempty"`        // "2.sg", "1.pl", "2.pl"
	PastParticiple  []string            `json:"past_participle,omitempty"`
	PassParticiple  []string            `json:"passive_participle,omitempty"`
	Transgressive   []string            `json:"transgressive,omitempty"`
}

// InflectionSummary provides ready-to-render grammar tables alongside raw forms.
type InflectionSummary struct {
	Declension  []CaseRow         `json:"declension,omitempty"`
	Conjugation *ConjugationTable `json:"conjugation,omitempty"`
	Degrees     map[string][]string `json:"degrees,omitempty"` // "1", "2", "3"
}

// LexemeDetail is the complete dictionary card for a single lexeme (homonym/POS).
type LexemeDetail struct {
	ID            int64              `json:"id"`
	Lemma         string             `json:"lemma"`
	POS           string             `json:"pos"`
	POSCs         string             `json:"pos_cs,omitempty"`
	Gender        string             `json:"gender,omitempty"`
	Animacy       string             `json:"animacy,omitempty"`
	Aspect        string             `json:"aspect,omitempty"`
	IPA           string             `json:"ipa,omitempty"`
	AudioURL      string             `json:"audio_url,omitempty"`
	Hyphenation   string             `json:"hyphenation,omitempty"`
	Syllables     []string           `json:"syllables,omitempty"`
	Etymology     string             `json:"etymology,omitempty"`
	Source        string             `json:"source"`
	Senses        []Sense            `json:"senses,omitempty"`
	Synonyms      []Relation         `json:"synonyms,omitempty"`
	Antonyms      []Relation         `json:"antonyms,omitempty"`
	Related       []Relation         `json:"related,omitempty"`
	Inflection    *InflectionSummary `json:"inflection,omitempty"`
	Forms                 []WordForm         `json:"forms,omitempty"`
	GeneratedForms        []string           `json:"generated_forms,omitempty"`
	BaseLemma             string             `json:"base_lemma,omitempty"`
	Translations          []Translation      `json:"translations,omitempty"`
	TranslationsFromLemma string             `json:"translations_from_lemma,omitempty"`
}

// FormMatch describes how an input query matches an inflected form of a lemma.
type FormMatch struct {
	Form      string   `json:"form"`
	LexemeID  int64    `json:"lexeme_id"`
	Lemma     string   `json:"lemma"`
	POS       string   `json:"pos"`
	POSCs     string   `json:"pos_cs,omitempty"`
	Case      string   `json:"case,omitempty"`
	CaseName  string   `json:"case_name,omitempty"`
	Number    string   `json:"number,omitempty"`
	Person    string   `json:"person,omitempty"`
	Tense     string   `json:"tense,omitempty"`
	Mood      string   `json:"mood,omitempty"`
	Voice     string   `json:"voice,omitempty"`
	Degree    string   `json:"degree,omitempty"`
	Gender    string   `json:"gender,omitempty"`
	Animacy   string   `json:"animacy,omitempty"`
	NonFinite string   `json:"nonfinite,omitempty"`
	Source    string   `json:"source"`
	Tags      []string `json:"tags,omitempty"`
}

// AutocompleteItem is a lightweight suggestion entry.
type AutocompleteItem struct {
	Lemma       string `json:"lemma"`
	POS         string `json:"pos,omitempty"`
	POSCs       string `json:"pos_cs,omitempty"`
	Hyphenation string `json:"hyphenation,omitempty"`
	Preview     string `json:"preview,omitempty"`
	Source      string `json:"source"`
}

// SearchHit represents a full-text search match across definitions and lemmas.
type SearchHit struct {
	LexemeID   int64  `json:"lexeme_id"`
	Lemma      string `json:"lemma"`
	POS        string `json:"pos,omitempty"`
	POSCs      string `json:"pos_cs,omitempty"`
	Definition string `json:"definition"`
}

// DataSourceInfo documents license & attribution metadata for each dataset.
type DataSourceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	License     string `json:"license"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
	RecordCount int64  `json:"record_count"`
	ImportedAt  string `json:"imported_at"`
}

// CaseName returns the Czech name of grammatical case "1".."7".
func CaseName(c string) string {
	switch c {
	case "1":
		return "nominativ"
	case "2":
		return "genitiv"
	case "3":
		return "dativ"
	case "4":
		return "akuzativ"
	case "5":
		return "vokativ"
	case "6":
		return "lokál"
	case "7":
		return "instrumentál"
	default:
		return ""
	}
}
