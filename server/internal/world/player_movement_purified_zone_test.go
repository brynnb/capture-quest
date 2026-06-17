package world

import "testing"

func TestIsPokemonTower5FPurifiedZone(t *testing.T) {
	tests := []struct {
		name    string
		mapName string
		x       int
		y       int
		want    bool
	}{
		{name: "top left", mapName: "POKEMON_TOWER_5F", x: 10, y: 8, want: true},
		{name: "top right", mapName: "POKEMON_TOWER_5F", x: 11, y: 8, want: true},
		{name: "bottom left", mapName: "POKEMON_TOWER_5F", x: 10, y: 9, want: true},
		{name: "bottom right", mapName: "POKEMON_TOWER_5F", x: 11, y: 9, want: true},
		{name: "outside left", mapName: "POKEMON_TOWER_5F", x: 9, y: 8, want: false},
		{name: "outside below", mapName: "POKEMON_TOWER_5F", x: 10, y: 10, want: false},
		{name: "wrong map", mapName: "POKEMON_TOWER_4F", x: 10, y: 8, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPokemonTower5FPurifiedZone(tt.mapName, tt.x, tt.y); got != tt.want {
				t.Fatalf("isPokemonTower5FPurifiedZone(%q, %d, %d) = %t, want %t", tt.mapName, tt.x, tt.y, got, tt.want)
			}
		})
	}
}
