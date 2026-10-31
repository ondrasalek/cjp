package hunspell

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestCzechHunspell(t *testing.T) {
	aff := filepath.Join("..", "..", "data", "raw", "cs_CZ.aff")
	dic := filepath.Join("..", "..", "data", "raw", "cs_CZ.dic")

	d, err := Load(aff, dic)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if d.StemsCount() < 200000 {
		t.Fatalf("expected >200k stems, got %d", d.StemsCount())
	}

	cases := []struct {
		word     string
		wantStem string
	}{
		{"kuchyňskými", "kuchyňský"},
		{"nejkrásnějšími", "krásnější"},
		{"nejnekrásnějšími", "krásnější"},
		{"Aakjaerovými", "Aakjaer"},
		{"nedobrého", "dobrý"},
		{"ženami", "žena"},
	}

	for _, tc := range cases {
		stems := d.Analyze(tc.word)
		if !slices.Contains(stems, tc.wantStem) {
			t.Errorf("Analyze(%q) = %v, expected to contain %q", tc.word, stems, tc.wantStem)
		}
	}

	// Non-existent nonsense word should return empty
	if got := d.Analyze("xyzneexistujicislovo"); len(got) != 0 {
		t.Errorf("Analyze(nonsense) = %v, expected empty", got)
	}
}
