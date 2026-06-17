package world

import "testing"

func TestFormatMapDisplayName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "town constant", raw: "PALLET_TOWN", want: "Pallet Town"},
		{name: "route constant", raw: "ROUTE_20", want: "Route 20"},
		{name: "floor constant", raw: "MT_MOON_1F", want: "Mt. Moon 1F"},
		{name: "camel case", raw: "CeruleanCity", want: "Cerulean City"},
		{name: "unified overworld", raw: "UNIFIED_OVERWORLD", want: "Kanto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatMapDisplayName(tt.raw); got != tt.want {
				t.Fatalf("formatMapDisplayName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
