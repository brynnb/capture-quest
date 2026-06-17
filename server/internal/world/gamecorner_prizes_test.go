package world

import "testing"

func TestFilterGameCornerPrizesForWindow(t *testing.T) {
	prizes := []GameCornerPrize{
		{ID: 1, Name: "ABRA"},
		{ID: 2, Name: "CLEFAIRY"},
		{ID: 3, Name: "NIDORINA"},
		{ID: 4, Name: "DRATINI"},
		{ID: 5, Name: "SCYTHER"},
		{ID: 6, Name: "PORYGON"},
		{ID: 7, Name: "TM23 Dragon Rage"},
		{ID: 8, Name: "TM15 Hyper Beam"},
		{ID: 9, Name: "TM50 Substitute"},
	}

	got := FilterGameCornerPrizesForWindow(prizes, 2)
	if len(got) != 3 {
		t.Fatalf("window 2 length = %d, want 3", len(got))
	}
	wantNames := []string{"DRATINI", "SCYTHER", "PORYGON"}
	for i, want := range wantNames {
		if got[i].Name != want {
			t.Fatalf("window 2 prize %d = %q, want %q", i, got[i].Name, want)
		}
		if got[i].PrizeWindow != 0 {
			t.Fatalf("manual fixture prizeWindow = %d, want untouched zero", got[i].PrizeWindow)
		}
	}
}
