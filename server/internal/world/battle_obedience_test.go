package world

import (
	"testing"

	"capturequest/internal/db"
)

func TestObedienceLevelForCharacterBadgeFlags(t *testing.T) {
	previousDB := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() {
		db.GlobalWorldDB = previousDB
	})

	charID := int64(987654)
	tests := []struct {
		name  string
		flags map[string]bool
		want  int
	}{
		{name: "none", flags: nil, want: 10},
		{name: "cascade", flags: map[string]bool{"EVENT_GOT_CASCADEBADGE": true}, want: 30},
		{name: "rainbow", flags: map[string]bool{"EVENT_GOT_RAINBOWBADGE": true}, want: 50},
		{name: "marsh", flags: map[string]bool{"EVENT_GOT_MARSHBADGE": true}, want: 70},
		{name: "earth", flags: map[string]bool{"EVENT_GOT_EARTHBADGE": true}, want: 100},
		{name: "defeated but no badge", flags: map[string]bool{"EVENT_BEAT_MISTY": true}, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			efm := &EventFlagManager{
				flags: map[int64]map[string]bool{
					charID: tt.flags,
				},
			}
			if got := obedienceLevelForCharacter(charID, efm); got != tt.want {
				t.Fatalf("obedienceLevelForCharacter() = %d, want %d", got, tt.want)
			}
		})
	}
}
