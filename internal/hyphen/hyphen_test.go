package hyphen

import (
	"path/filepath"
	"testing"
)

func TestCzechHyphenation(t *testing.T) {
	dictPath := filepath.Join("..", "..", "data", "raw", "hyph_cs_CZ.dic")
	h, err := LoadFile(dictPath)
	if err != nil {
		t.Fatalf("LoadFile(%s) failed: %v", dictPath, err)
	}

	cases := map[string]string{
		"čokoláda":   "čo-ko-lá-da",
		"lokomotiva": "lo-ko-mo-ti-va",
		"maminka":    "ma-min-ka",
		"učitelka":   "uči-tel-ka", // leftMin=2 prevents single-letter "u-" orphan in TeX standard
		"kuchyňský":  "ku-chyň-ský",
		"astronaut":  "as-tro-naut",
		"dobrý":      "dob-rý",
		"kapesník":   "ka-pes-ník",
		"vrba":       "vr-ba",
		"pes":        "pes",
	}

	for word, want := range cases {
		got := h.Hyphenate(word, "-")
		if got != want {
			t.Errorf("Hyphenate(%q) = %q, want %q", word, got, want)
		}
	}
}
