package world

import "testing"

func TestEndSafariSessionIfLeavingMap(t *testing.T) {
	const charID int64 = 42
	wh := &WorldHandler{Safari: NewSafariZoneManager()}
	wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	})

	if !endSafariSessionIfLeavingMap(charID, 220, 1, wh) {
		t.Fatal("expected safari session to end when leaving Safari Zone")
	}
	if session := wh.Safari.GetSession(charID); session != nil {
		t.Fatalf("expected no safari session after leaving, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapPreservesSafariGateExit(t *testing.T) {
	const charID int64 = 42
	wh := &WorldHandler{Safari: NewSafariZoneManager()}
	wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	})

	if endSafariSessionIfLeavingMap(charID, 220, SafariZoneGateMapID, wh) {
		t.Fatal("did not expect safari session to end when entering Safari Zone gate")
	}
	if session := wh.Safari.GetSession(charID); session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain for gate exit script, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapPreservesSafariToSafari(t *testing.T) {
	const charID int64 = 42
	wh := &WorldHandler{Safari: NewSafariZoneManager()}
	wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	})

	if endSafariSessionIfLeavingMap(charID, 220, 217, wh) {
		t.Fatal("did not expect safari session to end between Safari Zone maps")
	}
	if session := wh.Safari.GetSession(charID); session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapIgnoresNonSafariSource(t *testing.T) {
	const charID int64 = 42
	wh := &WorldHandler{Safari: NewSafariZoneManager()}
	wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	})

	if endSafariSessionIfLeavingMap(charID, 156, 220, wh) {
		t.Fatal("did not expect safari session to end before entering Safari Zone")
	}
	if session := wh.Safari.GetSession(charID); session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain, got %+v", session)
	}
}

func TestSafariGateEntryWarpRequiresActiveSession(t *testing.T) {
	const charID int64 = 42
	movement := &PlayerMovementManager{
		wh: &WorldHandler{Safari: NewSafariZoneManager()},
	}

	if !movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("expected Safari Zone entry warp to be blocked without an active session")
	}
}

func TestSafariGateEntryWarpAllowsActiveSession(t *testing.T) {
	const charID int64 = 42
	wh := &WorldHandler{Safari: NewSafariZoneManager()}
	wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	})
	movement := &PlayerMovementManager{wh: wh}

	if movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("did not expect Safari Zone entry warp to be blocked with an active session")
	}
}

func TestSafariGateEntryWarpIgnoresNonSafariDestination(t *testing.T) {
	const charID int64 = 42
	movement := &PlayerMovementManager{
		wh: &WorldHandler{Safari: NewSafariZoneManager()},
	}

	if movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, 1, nil) {
		t.Fatal("did not expect non-Safari destination to be blocked")
	}
}
