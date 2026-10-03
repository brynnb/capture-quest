package world

import "testing"

func TestEndSafariSessionIfLeavingMap(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}

	if ended, err := endSafariSessionIfLeavingMap(charID, 220, 1, wh); err != nil || !ended {
		t.Fatal("expected safari session to end when leaving Safari Zone")
	}
	if session, err := wh.Safari.GetSession(charID); err != nil || session != nil {
		t.Fatalf("expected no safari session after leaving, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapPreservesSafariGateExit(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}

	if ended, err := endSafariSessionIfLeavingMap(charID, 220, SafariZoneGateMapID, wh); err != nil || ended {
		t.Fatal("did not expect safari session to end when entering Safari Zone gate")
	}
	if session, err := wh.Safari.GetSession(charID); err != nil || session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain for gate exit script, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapPreservesSafariToSafari(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}

	if ended, err := endSafariSessionIfLeavingMap(charID, 220, 217, wh); err != nil || ended {
		t.Fatal("did not expect safari session to end between Safari Zone maps")
	}
	if session, err := wh.Safari.GetSession(charID); err != nil || session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain, got %+v", session)
	}
}

func TestEndSafariSessionIfLeavingMapIgnoresNonSafariSource(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}

	if ended, err := endSafariSessionIfLeavingMap(charID, 156, 220, wh); err != nil || ended {
		t.Fatal("did not expect safari session to end before entering Safari Zone")
	}
	if session, err := wh.Safari.GetSession(charID); err != nil || session == nil || !session.Active {
		t.Fatalf("expected active safari session to remain, got %+v", session)
	}
}

func TestSafariGateEntryWarpRequiresActiveSession(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	movement := &PlayerMovementManager{wh: wh}

	if !movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("expected Safari Zone entry warp to be blocked without an active session")
	}
}

func TestSafariGateEntryWarpAllowsActiveSession(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.SetSession(charID, SafariSession{
		Active:    true,
		BallsLeft: 30,
		StepsLeft: 500,
	}); err != nil {
		t.Fatal(err)
	}
	movement := &PlayerMovementManager{wh: wh}

	if movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, SafariZoneCenterMapID, nil) {
		t.Fatal("did not expect Safari Zone entry warp to be blocked with an active session")
	}
}

func TestSafariGateEntryWarpIgnoresNonSafariDestination(t *testing.T) {
	const charID int64 = 42
	_, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(wh.database)
	movement := &PlayerMovementManager{wh: wh}

	if movement.isSafariEntryWarpBlocked(charID, SafariZoneGateMapID, 1, nil) {
		t.Fatal("did not expect non-Safari destination to be blocked")
	}
}
