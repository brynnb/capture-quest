package world

import "testing"

func TestCanReachItemForPickup(t *testing.T) {
	item := itemPickupTile{X: 10, Y: 20, MapID: 65}
	tests := []struct {
		name   string
		player itemPickupTile
		want   bool
	}{
		{name: "above", player: itemPickupTile{X: 10, Y: 19, MapID: 65}, want: true},
		{name: "below", player: itemPickupTile{X: 10, Y: 21, MapID: 65}, want: true},
		{name: "left", player: itemPickupTile{X: 9, Y: 20, MapID: 65}, want: true},
		{name: "right", player: itemPickupTile{X: 11, Y: 20, MapID: 65}, want: true},
		{name: "same tile", player: itemPickupTile{X: 10, Y: 20, MapID: 65}, want: false},
		{name: "diagonal", player: itemPickupTile{X: 11, Y: 21, MapID: 65}, want: false},
		{name: "too far", player: itemPickupTile{X: 10, Y: 22, MapID: 65}, want: false},
		{name: "different map", player: itemPickupTile{X: 10, Y: 19, MapID: 66}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canReachItemForPickup(tt.player, item); got != tt.want {
				t.Fatalf("canReachItemForPickup() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestItemPickupMessage(t *testing.T) {
	if got, want := itemPickupMessage("POTION"), "Picked up POTION."; got != want {
		t.Fatalf("itemPickupMessage() = %q, want %q", got, want)
	}
	if got, want := itemPickupMessage(""), "Picked up item."; got != want {
		t.Fatalf("itemPickupMessage(empty) = %q, want %q", got, want)
	}
}
