package world

import "testing"

func TestApplyDefaultSpawnUsesRedsHouse2F(t *testing.T) {
	profile := &CharacterCreateProfile{}

	ApplyDefaultSpawn(profile)

	if got, want := profile.MapID, int32(38); got != want {
		t.Fatalf("MapID = %d, want %d", got, want)
	}
	if got, want := profile.X, float32(3); got != want {
		t.Fatalf("X = %f, want %f", got, want)
	}
	if got, want := profile.Y, float32(6); got != want {
		t.Fatalf("Y = %f, want %f", got, want)
	}
	if got, want := initialPlayerDirection(int(profile.MapID), int(profile.X), int(profile.Y)), "UP"; got != want {
		t.Fatalf("initialPlayerDirection = %q, want %q", got, want)
	}
}

func TestInitialPlayerDirectionDefaultsDownAwayFromSpawn(t *testing.T) {
	if got, want := initialPlayerDirection(DefaultSpawnMap, int(DefaultSpawnX), int(DefaultSpawnY)+1), "DOWN"; got != want {
		t.Fatalf("initialPlayerDirection away from spawn = %q, want %q", got, want)
	}
}

func TestRecoverySpawnUsesPalletTownOutsideTile(t *testing.T) {
	if got, want := RecoverySpawnMap, PalletTownMapID; got != want {
		t.Fatalf("RecoverySpawnMap = %d, want %d", got, want)
	}
	if got, want := RecoverySpawnX, 9.0; got != want {
		t.Fatalf("RecoverySpawnX = %f, want %f", got, want)
	}
	if got, want := RecoverySpawnY, 4.0; got != want {
		t.Fatalf("RecoverySpawnY = %f, want %f", got, want)
	}
}
