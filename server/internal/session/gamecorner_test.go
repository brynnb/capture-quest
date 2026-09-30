package session

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGameCornerDrawOwnershipAndMapReset(t *testing.T) {
	var state GameCornerState
	var draws atomic.Int32
	draw := func() (int, error) { return int(draws.Add(1)), nil }
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := state.LuckyIndex(42, 135, draw)
			if err != nil || value != 1 {
				t.Errorf("draw %d %v", value, err)
			}
		}()
	}
	wg.Wait()
	state.ObserveMap(135)
	if value, err := state.LuckyIndex(42, 135, draw); err != nil || value != 1 || draws.Load() != 1 {
		t.Fatalf("modal reopened rerolled: %d %v", value, err)
	}
	state.ObserveMap(137)
	if value, err := state.LuckyIndex(42, 135, draw); err != nil || value != 2 {
		t.Fatalf("map reentry %d %v", value, err)
	}
	if value, err := state.LuckyIndex(43, 135, draw); err != nil || value != 3 {
		t.Fatalf("character change %d %v", value, err)
	}
	state.Clear()
	if value, err := state.LuckyIndex(43, 135, draw); err != nil || value != 4 {
		t.Fatalf("cleared owner %d %v", value, err)
	}
}
