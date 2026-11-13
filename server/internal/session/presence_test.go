package session

import (
	"context"
	"testing"
)

func TestPresencePublishesCoherentValuesAcrossCommands(t *testing.T) {
	s := &Session{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 1; i <= 1000; i++ {
			_ = s.ExecuteCommand(context.Background(), func() { s.X = float32(i); s.Y = float32(i); s.MapID = i })
		}
	}()
	for {
		p := s.Presence()
		if p.X != p.Y || p.X != float32(p.MapID) {
			t.Errorf("torn presence: %+v", p)
			<-done
			return
		}
		select {
		case <-done:
			return
		default:
		}
	}
}
