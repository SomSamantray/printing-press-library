package report

import "testing"

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
