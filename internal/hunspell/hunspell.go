package hunspell

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

type condToken struct {
	any     bool
	negated bool
	chars   []rune
}

func (c condToken) matches(r rune) bool {
	if c.any {
		return true
	}
	found := false
	for _, ch := range c.chars {
		if ch == r {
			found = true
			break
		}
	}
	if c.negated {
		return !found
	}
	return found
}

func parseCondition(cond string) []condToken {
	if cond == "" || cond == "." {
		return nil
	}
	runes := []rune(cond)
	var tokens []condToken
	for i := 0; i < len(runes); {
		switch runes[i] {
		case '.':
			tokens = append(tokens, condToken{any: true})
			i++
		case '[':
			i++
			neg := false
			if i < len(runes) && runes[i] == '^' {
				neg = true
				i++
			}
			var chars []rune
			for i < len(runes) && runes[i] != ']' {
				chars = append(chars, runes[i])
				i++
			}
			if i < len(runes) && runes[i] == ']' {
				i++
			}
			tokens = append(tokens, condToken{negated: neg, chars: chars})
		default:
			tokens = append(tokens, condToken{chars: []rune{runes[i]}})
			i++
		}
	}
	return tokens
}

type affixRule struct {
	flag      rune
	cross     bool
	strip     []rune
	add       []rune
	addStr    string
	contFlags string
	cond      []condToken
}

func (r *affixRule) matchSuffix(stem []rune) bool {
	if len(r.cond) > len(stem) {
		return false
	}
	offset := len(stem) - len(r.cond)
	for i, tok := range r.cond {
		if !tok.matches(stem[offset+i]) {
			return false
		}
	}
	if len(r.strip) > len(stem) {
		return false
	}
	sOff := len(stem) - len(r.strip)
	for i, sr := range r.strip {
		if stem[sOff+i] != sr {
			return false
		}
	}
	return true
}

func (r *affixRule) matchPrefix(stem []rune) bool {
	if len(r.cond) > len(stem) {
		return false
	}
	for i, tok := range r.cond {
		if !tok.matches(stem[i]) {
			return false
		}
	}
	if len(r.strip) > len(stem) {
		return false
	}
	for i, sr := range r.strip {
		if stem[i] != sr {
			return false
		}
	}
	return true
}

// Entry represents a stem and its flags in a Hunspell .dic file.
type Entry struct {
	Stem  string
	Flags string
}

// Dictionary provides fast Hunspell stem lookup, form generation, and morphological validation.
type Dictionary struct {
	forbiddenFlag rune
	noSuggestFlag rune
	prefixes      map[rune][]affixRule
	suffixes      map[rune][]affixRule
	allPrefixes   []affixRule
	bySuffixEnd   map[rune][]affixRule // indexed by last rune of add (or 0 if add is empty)
	emptyAddSfx   []affixRule
	stems         map[string][]string // stem -> list of flag strings (homographs can have multiple lines)
	lowerIndex    map[string][]string // lowercase stem -> original case stems
}

// Load loads a Hunspell .aff and .dic pair (UTF-8).
func Load(affPath, dicPath string) (*Dictionary, error) {
	d := &Dictionary{
		prefixes:    make(map[rune][]affixRule),
		suffixes:    make(map[rune][]affixRule),
		bySuffixEnd: make(map[rune][]affixRule),
		stems:       make(map[string][]string, 270000),
		lowerIndex:  make(map[string][]string, 270000),
	}
	if err := d.loadAff(affPath); err != nil {
		return nil, err
	}
	if err := d.loadDic(dicPath); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dictionary) loadAff(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open aff: %w", err)
	}
	defer f.Close()

	crossMap := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "FORBIDDENWORD":
			r := []rune(fields[1])
			if len(r) > 0 {
				d.forbiddenFlag = r[0]
			}
		case "NOSUGGEST":
			r := []rune(fields[1])
			if len(r) > 0 {
				d.noSuggestFlag = r[0]
			}
		case "PFX", "SFX":
			if len(fields) == 4 && (fields[2] == "Y" || fields[2] == "N") {
				crossMap[fields[0]+":"+fields[1]] = (fields[2] == "Y")
				continue
			}
			if len(fields) < 5 {
				continue
			}
			flagRunes := []rune(fields[1])
			if len(flagRunes) == 0 {
				continue
			}
			flag := flagRunes[0]
			stripStr := fields[2]
			if stripStr == "0" {
				stripStr = ""
			}
			addRaw := fields[3]
			contFlags := ""
			if slash := strings.IndexByte(addRaw, '/'); slash >= 0 {
				contFlags = addRaw[slash+1:]
				addRaw = addRaw[:slash]
			}
			if addRaw == "0" {
				addRaw = ""
			}
			rule := affixRule{
				flag:      flag,
				cross:     crossMap[fields[0]+":"+fields[1]],
				strip:     []rune(stripStr),
				add:       []rune(addRaw),
				addStr:    addRaw,
				contFlags: contFlags,
				cond:      parseCondition(fields[4]),
			}
			if fields[0] == "PFX" {
				d.prefixes[flag] = append(d.prefixes[flag], rule)
				d.allPrefixes = append(d.allPrefixes, rule)
			} else {
				d.suffixes[flag] = append(d.suffixes[flag], rule)
				if len(rule.add) == 0 {
					d.emptyAddSfx = append(d.emptyAddSfx, rule)
				} else {
					last := rule.add[len(rule.add)-1]
					d.bySuffixEnd[last] = append(d.bySuffixEnd[last], rule)
				}
			}
		}
	}
	return scanner.Err()
}

func (d *Dictionary) loadDic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dic: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			// First line in .dic is entry count
			if isAllDigits(line) {
				continue
			}
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional tab/morph metadata if any
		if tab := strings.IndexByte(line, '\t'); tab >= 0 {
			line = line[:tab]
		}
		stem := line
		flags := ""
		if slash := strings.IndexByte(line, '/'); slash >= 0 {
			stem = line[:slash]
			flags = line[slash+1:]
		}
		if stem == "" {
			continue
		}
		if d.forbiddenFlag != 0 && strings.ContainsRune(flags, d.forbiddenFlag) {
			continue
		}
		d.stems[stem] = append(d.stems[stem], flags)
		low := strings.ToLower(stem)
		seen := false
		for _, ex := range d.lowerIndex[low] {
			if ex == stem {
				seen = true
				break
			}
		}
		if !seen {
			d.lowerIndex[low] = append(d.lowerIndex[low], stem)
		}
	}
	return scanner.Err()
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// StemsCount returns the number of unique valid stems in the dictionary.
func (d *Dictionary) StemsCount() int {
	return len(d.stems)
}

// ForEachStem iterates over all non-forbidden stems in the dictionary.
func (d *Dictionary) ForEachStem(fn func(stem string, flagSets []string) error) error {
	for stem, flags := range d.stems {
		if err := fn(stem, flags); err != nil {
			return err
		}
	}
	return nil
}

// Expand generates all valid inflected forms for a dictionary stem.
func (d *Dictionary) Expand(stem string) []string {
	flagSets, ok := d.stems[stem]
	if !ok {
		if cands, ok2 := d.lowerIndex[strings.ToLower(stem)]; ok2 && len(cands) > 0 {
			stem = cands[0]
			flagSets = d.stems[stem]
		} else {
			return nil
		}
	}

	set := make(map[string]struct{}, 32)
	set[stem] = struct{}{}
	stemRunes := []rune(stem)

	for _, flags := range flagSets {
		d.expandWithFlags(stemRunes, flags, true, set)
	}

	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (d *Dictionary) expandWithFlags(stem []rune, flags string, allowPrefix bool, out map[string]struct{}) {
	flagRunes := []rune(flags)

	type pfxResult struct {
		word  []rune
		cross bool
	}
	var prefixed []pfxResult
	if allowPrefix {
		for _, fl := range flagRunes {
			for _, pr := range d.prefixes[fl] {
				if pr.matchPrefix(stem) {
					w := make([]rune, 0, len(pr.add)+len(stem)-len(pr.strip))
					w = append(w, pr.add...)
					w = append(w, stem[len(pr.strip):]...)
					out[string(w)] = struct{}{}
					prefixed = append(prefixed, pfxResult{word: w, cross: pr.cross})
				}
			}
		}
	}

	for _, fl := range flagRunes {
		for _, sr := range d.suffixes[fl] {
			if sr.matchSuffix(stem) {
				base := stem[:len(stem)-len(sr.strip)]
				w := make([]rune, 0, len(base)+len(sr.add))
				w = append(w, base...)
				w = append(w, sr.add...)
				out[string(w)] = struct{}{}

				if sr.contFlags != "" {
					d.expandWithFlags(w, sr.contFlags, false, out)
				}

				if sr.cross {
					for _, pf := range prefixed {
						if pf.cross && sr.matchSuffix(pf.word) {
							pBase := pf.word[:len(pf.word)-len(sr.strip)]
							pw := make([]rune, 0, len(pBase)+len(sr.add))
							pw = append(pw, pBase...)
							pw = append(pw, sr.add...)
							out[string(pw)] = struct{}{}
							if sr.contFlags != "" {
								d.expandWithFlags(pw, sr.contFlags, false, out)
							}
						}
					}
				}
			}
		}
	}
}

// Analyze finds all dictionary stems (lemmas) capable of producing word.
// Every candidate stem is forward-verified via Expand() for 100% precision.
func (d *Dictionary) Analyze(word string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}

	candidates := make(map[string]struct{})
	addCandidate := func(cand string) {
		if _, ok := d.stems[cand]; ok {
			candidates[cand] = struct{}{}
		}
		if list, ok := d.lowerIndex[strings.ToLower(cand)]; ok {
			for _, s := range list {
				candidates[s] = struct{}{}
			}
		}
	}

	// 1. Direct match
	addCandidate(word)

	// Prepare variants: original word and lowercase word (if capitalized at sentence start)
	variants := []string{word}
	low := strings.ToLower(word)
	if low != word {
		variants = append(variants, low)
	}

	for _, v := range variants {
		vRunes := []rune(v)
		// Reverse without prefix + with single prefix
		d.collectSuffixCandidates(vRunes, addCandidate)

		for _, pr := range d.allPrefixes {
			if len(pr.add) > 0 && len(vRunes) > len(pr.add) && hasPrefixRunes(vRunes, pr.add) {
				unprefixed := make([]rune, 0, len(pr.strip)+len(vRunes)-len(pr.add))
				unprefixed = append(unprefixed, pr.strip...)
				unprefixed = append(unprefixed, vRunes[len(pr.add):]...)
				addCandidate(string(unprefixed))
				d.collectSuffixCandidates(unprefixed, addCandidate)
			}
		}
	}

	// Forward-verify every candidate stem against word (or lowercase match for capitalized input)
	var verified []string
	isUpperStart := len(word) > 0 && unicode.IsUpper([]rune(word)[0])
	for cand := range candidates {
		forms := d.Expand(cand)
		matched := false
		for _, f := range forms {
			if f == word || (isUpperStart && strings.EqualFold(f, word) && !unicode.IsUpper([]rune(cand)[0])) {
				matched = true
				break
			}
		}
		if matched {
			verified = append(verified, cand)
		}
	}
	sort.Strings(verified)
	return verified
}

func (d *Dictionary) collectSuffixCandidates(word []rune, addCandidate func(string)) {
	if len(word) == 0 {
		return
	}
	last := word[len(word)-1]
	rules := d.bySuffixEnd[last]

	tryReverse := func(sr affixRule, target []rune, allowSecondLevel bool) {
		if len(target) < len(sr.add) || !hasSuffixRunes(target, sr.add) {
			return
		}
		stem := make([]rune, 0, len(target)-len(sr.add)+len(sr.strip))
		stem = append(stem, target[:len(target)-len(sr.add)]...)
		stem = append(stem, sr.strip...)
		if len(stem) == 0 {
			return
		}
		if sr.matchSuffix(stem) {
			addCandidate(string(stem))
		}
		// Handle 2-stage continuation suffixes (e.g. Aakjaer -> Aakjaerův/Y -> Aakjaerovými)
		if allowSecondLevel && len(stem) > 0 {
			prevLast := stem[len(stem)-1]
			for _, sr1 := range d.bySuffixEnd[prevLast] {
				if sr1.contFlags != "" && strings.ContainsRune(sr1.contFlags, sr.flag) {
					if len(stem) >= len(sr1.add) && hasSuffixRunes(stem, sr1.add) {
						stem0 := make([]rune, 0, len(stem)-len(sr1.add)+len(sr1.strip))
						stem0 = append(stem0, stem[:len(stem)-len(sr1.add)]...)
						stem0 = append(stem0, sr1.strip...)
						if len(stem0) > 0 && sr1.matchSuffix(stem0) {
							addCandidate(string(stem0))
						}
					}
				}
			}
		}
	}

	for _, sr := range rules {
		tryReverse(sr, word, true)
	}
	for _, sr := range d.emptyAddSfx {
		tryReverse(sr, word, false)
	}
}

func hasPrefixRunes(s, prefix []rune) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i, r := range prefix {
		if s[i] != r {
			return false
		}
	}
	return true
}

func hasSuffixRunes(s, suffix []rune) bool {
	if len(suffix) > len(s) {
		return false
	}
	off := len(s) - len(suffix)
	for i, r := range suffix {
		if s[off+i] != r {
			return false
		}
	}
	return true
}
