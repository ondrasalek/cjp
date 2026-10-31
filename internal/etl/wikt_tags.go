package etl

import "strings"

// ParsedMorph holds normalized grammatical features for a word form.
type ParsedMorph struct {
	Case      string
	Number    string
	Person    string
	Tense     string
	Mood      string
	Voice     string
	Degree    string
	Gender    string
	Animacy   string
	NonFinite string
	ExtraTags []string
}

// NormalizePOS maps Wiktionary/UniMorph POS strings to canonical POS + Czech label.
func NormalizePOS(pos, posTitle string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "noun", "n":
		return "noun", firstNonEmpty(posTitle, "podstatné jméno")
	case "verb", "v":
		return "verb", firstNonEmpty(posTitle, "sloveso")
	case "adj":
		return "adj", firstNonEmpty(posTitle, "přídavné jméno")
	case "adv":
		return "adv", firstNonEmpty(posTitle, "příslovce")
	case "pron":
		return "pron", firstNonEmpty(posTitle, "zájmeno")
	case "num":
		return "num", firstNonEmpty(posTitle, "číslovka")
	case "prep":
		return "prep", firstNonEmpty(posTitle, "předložka")
	case "conj":
		return "conj", firstNonEmpty(posTitle, "spojka")
	case "particle":
		return "particle", firstNonEmpty(posTitle, "částice")
	case "intj":
		return "intj", firstNonEmpty(posTitle, "citoslovce")
	case "phrase":
		return "phrase", firstNonEmpty(posTitle, "slovní spojení")
	case "proverb":
		return "proverb", firstNonEmpty(posTitle, "přísloví")
	case "abbrev":
		return "abbrev", firstNonEmpty(posTitle, "zkratka")
	case "prefix":
		return "prefix", firstNonEmpty(posTitle, "předpona")
	case "suffix":
		return "suffix", firstNonEmpty(posTitle, "přípona")
	case "name":
		return "name", firstNonEmpty(posTitle, "vlastní jméno")
	default:
		return strings.ToLower(pos), posTitle
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}

// ParseLexemeTags extracts gender, animacy, and aspect from headword tags.
func ParseLexemeTags(tags, rawTags []string) (gender, animacy, aspect string) {
	for _, t := range tags {
		switch t {
		case "masculine":
			gender = mergeSlash(gender, "m")
		case "feminine":
			gender = mergeSlash(gender, "f")
		case "neuter":
			gender = mergeSlash(gender, "n")
		case "animate":
			animacy = mergeSlash(animacy, "anim")
		case "inanimate":
			animacy = mergeSlash(animacy, "inan")
		case "imperfect", "imperfective":
			aspect = mergeSlash(aspect, "impf")
		case "perfect", "perfective":
			aspect = mergeSlash(aspect, "pf")
		case "biaspectual":
			aspect = "biasp"
		}
	}
	for _, rt := range rawTags {
		low := strings.ToLower(rt)
		switch {
		case strings.Contains(low, "nedokonav"):
			aspect = mergeSlash(aspect, "impf")
		case strings.Contains(low, "dokonav"):
			aspect = mergeSlash(aspect, "pf")
		case strings.Contains(low, "obouvid"):
			aspect = "biasp"
		}
	}
	return gender, animacy, aspect
}

func mergeSlash(cur, val string) string {
	if cur == "" || cur == val {
		return val
	}
	parts := strings.Split(cur, "/")
	for _, p := range parts {
		if p == val {
			return cur
		}
	}
	return cur + "/" + val
}

// ParseWiktionaryFormTags maps Kaikki/Wiktionary form tags into ParsedMorph.
func ParseWiktionaryFormTags(tags []string) (ParsedMorph, bool) {
	var m ParsedMorph
	for _, t := range tags {
		switch t {
		case "table-tags", "inflection-template", "class":
			return m, false
		case "nominative":
			m.Case = "1"
		case "genitive":
			m.Case = "2"
		case "dative":
			m.Case = "3"
		case "accusative":
			m.Case = "4"
		case "vocative":
			m.Case = "5"
		case "locative":
			m.Case = "6"
		case "instrumental":
			m.Case = "7"
		case "singular":
			m.Number = "sg"
		case "plural":
			m.Number = "pl"
		case "dual":
			m.Number = "du"
		case "first-person":
			m.Person = "1"
		case "second-person":
			m.Person = "2"
		case "third-person":
			m.Person = "3"
		case "present":
			m.Tense = "pres"
		case "past":
			m.Tense = "past"
		case "future":
			m.Tense = "fut"
		case "indicative":
			m.Mood = "ind"
		case "imperative":
			m.Mood = "imp"
		case "conditional":
			m.Mood = "cond"
		case "active":
			m.Voice = "act"
		case "passive":
			m.Voice = "pass"
		case "positive":
			m.Degree = "1"
		case "comparative":
			m.Degree = "2"
		case "superlative":
			m.Degree = "3"
		case "masculine":
			m.Gender = mergeSlash(m.Gender, "m")
		case "feminine":
			m.Gender = mergeSlash(m.Gender, "f")
		case "neuter":
			m.Gender = mergeSlash(m.Gender, "n")
		case "animate":
			m.Animacy = mergeSlash(m.Animacy, "anim")
		case "inanimate":
			m.Animacy = mergeSlash(m.Animacy, "inan")
		case "infinitive":
			m.NonFinite = "inf"
		case "participle":
			m.NonFinite = "part"
		case "transgressive":
			m.NonFinite = "transgr"
		default:
			m.ExtraTags = append(m.ExtraTags, t)
		}
	}
	return m, true
}

// ParseUniMorphTags maps UniMorph feature bundle (e.g. "N;ESS;PL") to POS + ParsedMorph.
func ParseUniMorphTags(bundle string) (pos string, posCs string, m ParsedMorph) {
	parts := strings.Split(bundle, ";")
	for _, p := range parts {
		switch p {
		case "N":
			pos, posCs = "noun", "podstatné jméno"
		case "V":
			pos, posCs = "verb", "sloveso"
		case "ADJ":
			pos, posCs = "adj", "přídavné jméno"
		case "NOM":
			m.Case = "1"
		case "GEN":
			m.Case = "2"
		case "DAT":
			m.Case = "3"
		case "ACC":
			m.Case = "4"
		case "VOC":
			m.Case = "5"
		case "ESS", "LOC":
			m.Case = "6"
		case "INS":
			m.Case = "7"
		case "SG":
			m.Number = "sg"
		case "PL":
			m.Number = "pl"
		case "1":
			m.Person = "1"
		case "2":
			m.Person = "2"
		case "3":
			m.Person = "3"
		case "PRS":
			m.Tense = "pres"
		case "PST":
			m.Tense = "past"
		case "FUT":
			m.Tense = "fut"
		case "IND":
			m.Mood = "ind"
		case "IMP":
			m.Mood = "imp"
		case "COND":
			m.Mood = "cond"
		case "ACT":
			m.Voice = "act"
		case "PASS":
			m.Voice = "pass"
		case "MASC":
			m.Gender = mergeSlash(m.Gender, "m")
		case "FEM":
			m.Gender = mergeSlash(m.Gender, "f")
		case "NEUT":
			m.Gender = mergeSlash(m.Gender, "n")
		case "ANIM":
			m.Animacy = mergeSlash(m.Animacy, "anim")
		case "INAN":
			m.Animacy = mergeSlash(m.Animacy, "inan")
		case "NFIN":
			m.NonFinite = "inf"
		case "V.PTCP":
			pos, posCs = "verb", "sloveso"
			m.NonFinite = "part"
		case "V.CVB":
			pos, posCs = "verb", "sloveso"
			m.NonFinite = "transgr"
		default:
			m.ExtraTags = append(m.ExtraTags, p)
		}
	}
	return pos, posCs, m
}

// TranslateLabelCs converts common English Wiktionary register/domain tags to Czech.
func TranslateLabelCs(tag string) string {
	switch tag {
	case "colloquially", "colloquial":
		return "hovorově"
	case "obsolete":
		return "zastarale"
	case "archaic":
		return "archaicky"
	case "dialectal":
		return "nářečně"
	case "literary":
		return "knižně"
	case "poetic":
		return "básnicky"
	case "vulgar":
		return "vulgárně"
	case "pejorative":
		return "hanlivě"
	case "expressively", "expressive":
		return "expresivně"
	case "figuratively", "figurative":
		return "přeneseně"
	case "rare":
		return "řídce"
	case "slang":
		return "slangově"
	case "informal":
		return "neformálně"
	default:
		return tag
	}
}
