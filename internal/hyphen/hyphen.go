package hyphen

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

type trieNode struct {
	children map[rune]*trieNode
	weights  []int // weights[k] applies before the k-th character of the matched pattern
}

// Hyphenator implements Frank Liang's TeX/LibreOffice hyphenation algorithm.
type Hyphenator struct {
	root     *trieNode
	leftMin  int
	rightMin int
}

// LoadFile loads a LibreOffice/TeX hyphenation dictionary (such as hyph_cs_CZ.dic).
func LoadFile(path string) (*Hyphenator, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hyphenation file: %w", err)
	}
	return LoadBytes(raw)
}

// LoadBytes parses a hyphenation dictionary from raw bytes (UTF-8 or ISO8859-2).
func LoadBytes(raw []byte) (*Hyphenator, error) {
	firstNL := bytes.IndexByte(raw, '\n')
	if firstNL < 0 {
		return nil, fmt.Errorf("invalid hyphenation file: missing header")
	}

	header := strings.TrimSpace(string(raw[:firstNL]))
	body := raw[firstNL+1:]

	var reader io.Reader = bytes.NewReader(body)
	upperHeader := strings.ToUpper(header)
	if strings.Contains(upperHeader, "8859-2") {
		reader = charmap.ISO8859_2.NewDecoder().Reader(reader)
	}

	h := &Hyphenator{
		root:     &trieNode{children: make(map[rune]*trieNode)},
		leftMin:  2,
		rightMin: 2,
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "LEFTHYPHENMIN") {
			_, _ = fmt.Sscanf(line, "LEFTHYPHENMIN %d", &h.leftMin)
			continue
		}
		if strings.HasPrefix(line, "RIGHTHYPHENMIN") {
			_, _ = fmt.Sscanf(line, "RIGHTHYPHENMIN %d", &h.rightMin)
			continue
		}
		if strings.HasPrefix(line, "COMPOUNDLEFTHYPHENMIN") ||
			strings.HasPrefix(line, "COMPOUNDRIGHTHYPHENMIN") ||
			strings.HasPrefix(line, "NEXTLEVEL") {
			continue
		}
		h.insertPattern(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan hyphenation patterns: %w", err)
	}
	return h, nil
}

func (h *Hyphenator) insertPattern(pat string) {
	runes := []rune(pat)
	var letters []rune
	weights := []int{0}

	for _, r := range runes {
		if r >= '0' && r <= '9' {
			weights[len(weights)-1] = int(r - '0')
		} else {
			letters = append(letters, r)
			weights = append(weights, 0)
		}
	}
	if len(letters) == 0 {
		return
	}

	node := h.root
	for _, r := range letters {
		next, ok := node.children[r]
		if !ok {
			next = &trieNode{children: make(map[rune]*trieNode)}
			node.children[r] = next
		}
		node = next
	}
	node.weights = weights
}

// Parts splits a Czech word (or multi-part expression) into hyphenation syllables.
func (h *Hyphenator) Parts(word string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}
	// If it contains spaces or explicit hyphens, hyphenate alphabetic tokens individually.
	if strings.ContainsAny(word, " -") {
		if !strings.Contains(word, " ") {
			sub := strings.Split(word, "-")
			var out []string
			for _, s := range sub {
				if s != "" {
					out = append(out, h.hyphenateToken(s)...)
				}
			}
			return out
		}
		return []string{word}
	}
	return h.hyphenateToken(word)
}

// Hyphenate joins syllable parts with sep (typically "-" or "·").
func (h *Hyphenator) Hyphenate(word, sep string) string {
	parts := h.Parts(word)
	if len(parts) == 0 {
		return word
	}
	return strings.Join(parts, sep)
}

func (h *Hyphenator) hyphenateToken(token string) []string {
	origRunes := []rune(token)
	n := len(origRunes)
	if n < h.leftMin+h.rightMin {
		return []string{token}
	}
	for _, r := range origRunes {
		if !unicode.IsLetter(r) {
			return []string{token}
		}
	}

	// Build ".word." in lowercase
	work := make([]rune, n+2)
	work[0] = '.'
	for i, r := range origRunes {
		work[i+1] = unicode.ToLower(r)
	}
	work[n+1] = '.'

	// levels[i] corresponds to the position before work[i];
	// thus split before origRunes[k] (for 1 <= k < n) is levels[k+1].
	levels := make([]int, len(work)+1)

	for i := 0; i < len(work); i++ {
		node := h.root
		for j := i; j < len(work); j++ {
			next, ok := node.children[work[j]]
			if !ok {
				break
			}
			node = next
			if node.weights != nil {
				for k, w := range node.weights {
					if w > levels[i+k] {
						levels[i+k] = w
					}
				}
			}
		}
	}

	var parts []string
	start := 0
	for k := 1; k < n; k++ {
		if k < h.leftMin || n-k < h.rightMin {
			continue
		}
		if levels[k+1]%2 == 1 {
			if hasCzechSyllabicNucleus(origRunes[start:k]) && hasCzechSyllabicNucleus(origRunes[k:]) {
				parts = append(parts, string(origRunes[start:k]))
				start = k
			}
		}
	}
	parts = append(parts, string(origRunes[start:]))
	return parts
}

func hasCzechSyllabicNucleus(seg []rune) bool {
	for _, r := range seg {
		switch unicode.ToLower(r) {
		case 'a', 'e', 'i', 'o', 'u', 'y', 'á', 'é', 'í', 'ó', 'ú', 'ů', 'ý', 'ě', 'ä', 'ö', 'ü':
			return true
		}
	}
	if len(seg) >= 2 {
		for _, r := range seg {
			switch unicode.ToLower(r) {
			case 'r', 'l', 'm':
				return true
			}
		}
	}
	return false
}
