package db_character

import "testing"

func TestDefaultOptionsIncludeRivalName(t *testing.T) {
	opts := DefaultOptions()
	if opts.RivalName != DefaultRivalName {
		t.Fatalf("DefaultOptions().RivalName = %q, want %q", opts.RivalName, DefaultRivalName)
	}
}

func TestNormalizeRivalName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "", want: DefaultRivalName},
		{name: "  bLuE  ", want: "Blue"},
		{name: "G@a-r_y!", want: "Gary"},
		{name: "abcdefghijklmnop", want: "Abcdefghijkl"},
	}

	for _, tc := range tests {
		if got := NormalizeRivalName(tc.name); got != tc.want {
			t.Errorf("NormalizeRivalName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
