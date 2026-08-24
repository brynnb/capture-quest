package main

import "testing"

func TestResolveWarpDestinationUpdatesUsesDestinationWarpIndex(t *testing.T) {
	maps := map[int]importedMapInfo{
		0:  {Name: "PALLET_TOWN", IsOverworld: true},
		40: {Name: "OAKS_LAB", IsOverworld: false},
	}
	offsets := map[string]coordinateOffset{
		"pallettown": {X: 0, Y: 0},
	}
	events := []importedWarpEvent{
		{MapID: 0, MapName: "PalletTown", X: 5, Y: 5, DestWarpIndex: 1},
		{MapID: 0, MapName: "PalletTown", X: 13, Y: 5, DestWarpIndex: 1},
		{MapID: 0, MapName: "PalletTown", X: 12, Y: 11, DestWarpIndex: 2},
		{MapID: 40, MapName: "OaksLab", X: 4, Y: 11, DestWarpIndex: 3},
		{MapID: 40, MapName: "OaksLab", X: 5, Y: 11, DestWarpIndex: 3},
	}
	warps := []importedRuntimeWarp{
		{ID: 723, SourceMapID: 40, X: 4, Y: 11, DestinationMapID: 0, HasDestination: true},
		{ID: 724, SourceMapID: 40, X: 5, Y: 11, DestinationMapID: 0, HasDestination: true},
	}

	updates := resolveWarpDestinationUpdates(maps, offsets, events, warps)
	if len(updates) != 2 {
		t.Fatalf("got %d updates, want 2: %#v", len(updates), updates)
	}
	for _, update := range updates {
		if update.X != 12 || update.Y != 11 {
			t.Fatalf("warp %d resolved to (%d,%d), want (12,11)", update.WarpID, update.X, update.Y)
		}
	}
}

func TestResolveWarpDestinationUpdatesConvertsOverworldDestinationToGlobalCoordinates(t *testing.T) {
	maps := map[int]importedMapInfo{
		1:  {Name: "VIRIDIAN_CITY", IsOverworld: true},
		41: {Name: "VIRIDIAN_POKECENTER", IsOverworld: false},
	}
	offsets := map[string]coordinateOffset{
		"viridiancity": {X: -10, Y: -72},
	}
	events := []importedWarpEvent{
		{MapID: 1, MapName: "ViridianCity", X: 23, Y: 25, DestWarpIndex: 1},
		{MapID: 41, MapName: "ViridianPokecenter", X: 3, Y: 7, DestWarpIndex: 1},
	}
	warps := []importedRuntimeWarp{
		{ID: 377, SourceMapID: 41, X: 3, Y: 7, DestinationMapID: 1, HasDestination: true},
	}

	updates := resolveWarpDestinationUpdates(maps, offsets, events, warps)
	if len(updates) != 1 {
		t.Fatalf("got %d updates, want 1: %#v", len(updates), updates)
	}
	if updates[0].X != 13 || updates[0].Y != -47 {
		t.Fatalf("resolved to (%d,%d), want global Viridian coords (13,-47)", updates[0].X, updates[0].Y)
	}
}

func TestResolveWarpDestinationUpdatesViridianForestSouthGateExitToRoute2(t *testing.T) {
	maps := map[int]importedMapInfo{
		13: {Name: "ROUTE_2", IsOverworld: true},
		50: {Name: "VIRIDIAN_FOREST_SOUTH_GATE", IsOverworld: false},
	}
	offsets := map[string]coordinateOffset{
		"route2": {X: 0, Y: -144},
	}
	events := []importedWarpEvent{
		{MapID: 13, MapName: "Route2", X: 12, Y: 9, DestWarpIndex: 1},
		{MapID: 13, MapName: "Route2", X: 3, Y: 11, DestWarpIndex: 2},
		{MapID: 13, MapName: "Route2", X: 15, Y: 19, DestWarpIndex: 1},
		{MapID: 13, MapName: "Route2", X: 16, Y: 35, DestWarpIndex: 2},
		{MapID: 13, MapName: "Route2", X: 15, Y: 39, DestWarpIndex: 3},
		{MapID: 13, MapName: "Route2", X: 3, Y: 43, DestWarpIndex: 3},
		{MapID: 50, MapName: "ViridianForestSouthGate", X: 4, Y: 7, DestWarpIndex: 6},
		{MapID: 50, MapName: "ViridianForestSouthGate", X: 5, Y: 7, DestWarpIndex: 6},
	}
	warps := []importedRuntimeWarp{
		{ID: 769, SourceMapID: 50, X: 4, Y: 7, DestinationMapID: 13, HasDestination: true},
		{ID: 770, SourceMapID: 50, X: 5, Y: 7, DestinationMapID: 13, HasDestination: true},
	}

	updates := resolveWarpDestinationUpdates(maps, offsets, events, warps)
	if len(updates) != 2 {
		t.Fatalf("got %d updates, want 2: %#v", len(updates), updates)
	}
	for _, update := range updates {
		if update.X != 3 || update.Y != -101 {
			t.Fatalf("warp %d resolved to (%d,%d), want Route2 global coords (3,-101)", update.WarpID, update.X, update.Y)
		}
	}
}
