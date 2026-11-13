package world

import (
	"capturequest/internal/session"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestChatOwnerDeadlineStopsPoolWaitAndPublication(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	published := false
	wh.SetPublicChatSink(func(ChatMessageBroadcast) { published = true })
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ses.ExecuteCommand(ctx, func() { HandleSendChatMessage(ses, []byte(`{"text":"waiting"}`), wh) })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("owner deadline lost: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		lease.Close()
		<-done
		t.Fatal("chat query outlived owner")
	}
	if database.Stats().WaitCount == before {
		t.Fatal("chat query did not wait on pool")
	}
	lease.Close()
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM chat_messages`).Scan(&count); err != nil || count != 0 || published {
		t.Fatalf("cancelled chat escaped: rows=%d published=%v err=%v", count, published, err)
	}
}

func TestPlayerChatPersistsAndBroadcastsTheSameBoundedUnicodeText(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	session.InitSessionManager(session.NewSessionManager())
	var published []ChatMessageBroadcast
	wh.SetPublicChatSink(func(message ChatMessageBroadcast) { published = append(published, message) })
	payload, _ := json.Marshal(SendChatMessageRequest{Text: "  " + strings.Repeat("界🙂", 200) + "  "})
	if err := ses.ExecuteCommand(context.Background(), func() { HandleSendChatMessage(ses, payload, wh); HandleSendChatMessage(ses, payload, wh) }); err != nil {
		t.Fatal(err)
	}
	var stored string
	var count int
	if err := database.QueryRow(`SELECT count(*),min(text) FROM chat_messages`).Scan(&count, &stored); err != nil || count != 1 {
		t.Fatalf("stored chat=%d %v", count, err)
	}
	if len(published) != 1 || published[0].Text != stored || !utf8.ValidString(stored) || utf8.RuneCountInString(stored) != maxChatMessageLength {
		t.Fatalf("bounded chat mismatch: %#v stored runes=%d", published, utf8.RuneCountInString(stored))
	}
}

func TestHeartbeatRejectsMalformedPayloadWithoutRefreshingLifetime(t *testing.T) {
	_, _, ses, messages := battleTestWorld(t)
	before := time.Unix(1, 0)
	ses.RecordHeartbeat(before)
	for _, payload := range []string{`{}`, `{"timestamp":null}`, `{"timestamp":"bad"}`, `{"timestamp":-1}`, `{"timestamp":1,"unknown":true}`} {
		HandleHeartbeat(ses, []byte(payload), nil)
		if !ses.LastHeartbeat().Equal(before) || len(messages.streams) != 0 {
			t.Fatalf("invalid heartbeat refreshed lifetime: %s", payload)
		}
	}
	HandleHeartbeat(ses, []byte(`{"timestamp":1.5}`), nil)
	if !ses.LastHeartbeat().After(before) || len(messages.streams) != 1 {
		t.Fatal("valid heartbeat not acknowledged")
	}
}
