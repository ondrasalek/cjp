package textnorm

import "testing"

func TestStripDiacritics(t *testing.T) {
	cases := map[string]string{
		"Příliš žluťoučký kůň": "prilis zlutoucky kun",
		"úpěl ďábelské ódy":    "upel dabelske ody",
		"ŽENA":                 "zena",
		"pes":                  "pes",
		"  Čeština  ":          "cestina",
	}
	for in, want := range cases {
		if got := StripDiacritics(in); got != want {
			t.Errorf("StripDiacritics(%q) = %q, want %q", in, got, want)
		}
	}
}
