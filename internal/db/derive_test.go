package db

import (
	"slices"
	"testing"
)

func TestCandidateBaseWords(t *testing.T) {
	cases := []struct {
		word string
		want string
	}{
		{"horizontálnější", "horizontální"},
		{"horizontálněji", "horizontální"},
		{"horizontálně", "horizontální"},
		{"krásnější", "krásný"},
		{"automatičtější", "automatický"},
		{"lidštější", "lidský"},
		{"moudřejší", "moudrý"},
		{"nejrychlejší", "rychlejší"},
		{"rychlejší", "rychlý"},
		{"programování", "programovat"},
		{"programovat", "program"},
		{"kuchyňský", "kuchyně"},
		{"učitelka", "učitel"},
		{"matčin", "matka"},
		{"ženin", "žena"},
		{"lepší", "dobrý"},
		{"hůře", "špatně"},
	}

	for _, tc := range cases {
		got := CandidateBaseWords(tc.word)
		if !slices.Contains(got, tc.want) {
			t.Errorf("CandidateBaseWords(%q) = %v, want %q included", tc.word, got, tc.want)
		}
	}

	// Ensure short words like "paní" do not strip down to 2-char "pa"
	paniCands := CandidateBaseWords("paní")
	if slices.Contains(paniCands, "pa") {
		t.Errorf("CandidateBaseWords(\"paní\") unexpectedly produced short stem \"pa\": %v", paniCands)
	}
}
