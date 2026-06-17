package world

import "testing"

func TestTrainerCardBadgeFlagsUseBadgeObtainedFlags(t *testing.T) {
	want := []string{
		"EVENT_GOT_BOULDERBADGE",
		"EVENT_GOT_CASCADEBADGE",
		"EVENT_GOT_THUNDERBADGE",
		"EVENT_GOT_RAINBOWBADGE",
		"EVENT_GOT_SOULBADGE",
		"EVENT_GOT_MARSHBADGE",
		"EVENT_GOT_VOLCANOBADGE",
		"EVENT_GOT_EARTHBADGE",
	}

	if len(badgeFlags) != len(want) {
		t.Fatalf("badgeFlags length = %d, want %d", len(badgeFlags), len(want))
	}
	for i := range want {
		if badgeFlags[i] != want[i] {
			t.Fatalf("badgeFlags[%d] = %q, want %q", i, badgeFlags[i], want[i])
		}
	}
}
