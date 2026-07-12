package report

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/eafc"
)

func TestFormatEuros(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  string
	}{
		{name: "zero", value: 0, want: "€0"},
		{name: "millions", value: 30_000_000, want: "€30.00m"},
		{name: "thousands", value: 950_000, want: "€950k"},
		{name: "fractional thousands", value: 1_500, want: "€1.5k"},
		{name: "euros", value: 750, want: "€750"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatEuros(test.value); got != test.want {
				t.Fatalf("FormatEuros(%d) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestDecodeRosterInitializesEmptySlice(t *testing.T) {
	players, err := decodeRoster([]byte(`{"players":null}`))
	if err != nil {
		t.Fatalf("decodeRoster() error = %v", err)
	}
	if players == nil || len(players) != 0 {
		t.Fatalf("decodeRoster() = %#v, want non-nil empty slice", players)
	}
}

func TestEAMatchConsistent(t *testing.T) {
	cases := []struct {
		name   string
		club   string
		nat    string
		eaTeam string
		eaNat  string
		wantOK bool
	}{
		{"exact club", "SL Benfica", "Norway", "SL Benfica", "Norway", true},
		{"club affix variation", "Benfica", "", "SL Benfica", "", true},
		{"real madrid variation", "Real Madrid", "", "Real Madrid CF", "", true},
		{"missing tm club accepts", "", "", "SL Benfica", "", true},
		{"missing ea team accepts", "SL Benfica", "", "", "", true},
		{"clear mismatch rejected", "SL Benfica", "Norway", "Manchester City", "England", false},
		{"mismatch club rescued by nationality", "SL Benfica", "Norway", "Some Other FC", "Norway", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := &PlayerReport{Club: tc.club, Nationality: tc.nat}
			player := &eafc.Player{Team: tc.eaTeam, Nationality: tc.eaNat}
			detail, ok := eaMatchConsistent(report, player)
			if ok != tc.wantOK {
				t.Fatalf("eaMatchConsistent ok = %v (detail %q), want %v", ok, detail, tc.wantOK)
			}
			if !ok && detail == "" {
				t.Fatalf("rejected match must carry a detail reason")
			}
		})
	}
}
