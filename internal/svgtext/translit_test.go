package svgtext

import "testing"

func TestTransliterate(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Т-1 2,5 МВА", "T-1 2,5 MVA"},
		{"Журнал событий", "Zhurnal sobytiy"},
		{"СВ-110", "SV-110"},
		{"кВ", "kV"},
		{"already english", "already english"},
		{"Щётка", "Shchetka"},
		{"объём", "obem"},
	}
	for _, c := range cases {
		if got := Transliterate(c.in); got != c.want {
			t.Errorf("Transliterate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
