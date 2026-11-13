package world

import (
	"context"
	"fmt"
	"testing"
)

func TestSafariDestinationLifecycleUsesPositionTransaction(t *testing.T) {
	for _, destination := range []int{60, SafariZoneGateMapID, SafariZoneCenterMapID, 217} {
		t.Run(fmt.Sprint(destination), func(t *testing.T) {
			database, wh, _, _ := battleTestWorld(t)
			wh.Safari = NewSafariZoneManager(database)
			if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
				t.Fatal(err)
			}
			if err := commitPlayerPosition(context.Background(), database, 42, destination, 3, 4); err != nil {
				t.Fatal(err)
			}
			visit, err := wh.Safari.GetSession(context.Background(), 42)
			if err != nil {
				t.Fatal(err)
			}
			shouldRetain := destination == SafariZoneGateMapID || IsInSafariZone(destination)
			if (visit != nil) != shouldRetain {
				t.Fatalf("destination=%d visit=%+v", destination, visit)
			}
			var savedMap int
			if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&savedMap); err != nil || savedMap != destination {
				t.Fatalf("destination=%d saved=%d error=%v", destination, savedMap, err)
			}
		})
	}
}

func TestSafariGateEntryWarpRequiresActiveSession(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	movement := &PlayerMovementManager{wh: wh}

	if !movement.isSafariEntryWarpBlocked(context.Background(), charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("expected Safari Zone entry warp to be blocked without an active session")
	}
}

func TestSafariGateEntryWarpAllowsActiveSession(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(context.Background(), charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}
	movement := &PlayerMovementManager{wh: wh}

	if movement.isSafariEntryWarpBlocked(context.Background(), charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("did not expect Safari Zone entry warp to be blocked with an active session")
	}
}

func TestSafariGateEntryWarpIgnoresNonSafariDestination(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	movement := &PlayerMovementManager{wh: wh}

	if movement.isSafariEntryWarpBlocked(context.Background(), charID, SafariZoneGateMapID, 1, nil) {
		t.Fatal("did not expect non-Safari destination to be blocked")
	}
}
