package session

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIssuedEventClaimsAreBoundedAndSingleUse(t *testing.T) {
	var events IssuedEvents
	token, err := events.Issue(42, "reward", []byte("snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := events.Claim(43, "reward", token); ok {
		t.Fatal("foreign character")
	}
	if _, ok := events.Claim(42, "other", token); ok {
		t.Fatal("wrong label")
	}
	var claims atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := events.Claim(42, "reward", token); ok {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatalf("claims=%d", claims.Load())
	}
	events.Finish(token, false)
	payload, ok := events.Claim(42, "reward", token)
	if !ok || string(payload) != "snapshot" {
		t.Fatal("failed attempt not retryable")
	}
	events.Finish(token, true)
	if _, ok := events.Claim(42, "reward", token); ok {
		t.Fatal("committed event replayed")
	}
	for i := 0; i < 8; i++ {
		if _, err := events.Issue(42, "pending", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := events.Issue(42, "overflow", nil); err == nil {
		t.Fatal("unbounded pending events")
	}
	events.Clear()
	token, err = events.Issue(42, "expired", nil)
	if err != nil {
		t.Fatal(err)
	}
	events.entries[token].expires = time.Now().Add(-time.Second)
	if _, ok := events.Claim(42, "expired", token); ok {
		t.Fatal("expired event accepted")
	}
}
