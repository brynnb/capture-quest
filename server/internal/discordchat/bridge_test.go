package discordchat

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func signedRequest(t *testing.T, secret string, body string, timestamp time.Time) *http.Request {
	t.Helper()
	timestampText := strconv.FormatInt(timestamp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestampText))
	_, _ = mac.Write([]byte(body))
	request := httptest.NewRequest(http.MethodPost, "/api/discord/game-chat", strings.NewReader(body))
	request.Header.Set("X-Game-Chat-Timestamp", timestampText)
	request.Header.Set("X-Game-Chat-Signature", hex.EncodeToString(mac.Sum(nil)))
	return request
}

func TestSignedIngressPublishesMessage(t *testing.T) {
	secret := strings.Repeat("s", 32)
	var got Message
	bridge := &Bridge{secret: secret, publish: func(senderName, text string) error {
		got = Message{SenderName: senderName, Text: text}
		return nil
	}}
	body := `{"senderName":"Tester[Discord]","text":"hello"}`
	recorder := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(recorder, signedRequest(t, secret, body, time.Now()))
	if recorder.Code != http.StatusNoContent || got != (Message{SenderName: "Tester[Discord]", Text: "hello"}) {
		t.Fatalf("status=%d message=%#v", recorder.Code, got)
	}
}

func TestIngressRejectsInvalidOrStaleSignature(t *testing.T) {
	secret := strings.Repeat("s", 32)
	bridge := &Bridge{secret: secret, publish: func(string, string) error { return nil }}
	body := `{"senderName":"Tester","text":"hello"}`
	for _, request := range []*http.Request{
		signedRequest(t, "wrong"+secret, body, time.Now()),
		signedRequest(t, secret, body, time.Now().Add(-6*time.Minute)),
	} {
		recorder := httptest.NewRecorder()
		bridge.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d, want unauthorized", recorder.Code)
		}
	}
}

func TestEnvironmentConfigurationIsAllOrNothing(t *testing.T) {
	t.Setenv("DISCORD_CHAT_SHARED_SECRET", "")
	t.Setenv("DISCORD_CHAT_WEBHOOK_URL", "")
	publish := func(string, string) error { return nil }

	bridge, err := NewFromEnvironment(publish)
	if err != nil || bridge != nil {
		t.Fatalf("disabled configuration = (%#v, %v), want (nil, nil)", bridge, err)
	}

	t.Setenv("DISCORD_CHAT_WEBHOOK_URL", "https://discord.example/webhook")
	if _, err := NewFromEnvironment(publish); err == nil {
		t.Fatal("partial configuration was accepted")
	}

	t.Setenv("DISCORD_CHAT_SHARED_SECRET", "too-short")
	if _, err := NewFromEnvironment(publish); err == nil {
		t.Fatal("short shared secret was accepted")
	}

	t.Setenv("DISCORD_CHAT_SHARED_SECRET", strings.Repeat("s", 32))
	bridge, err = NewFromEnvironment(publish)
	if err != nil || bridge == nil {
		t.Fatalf("complete configuration = (%#v, %v), want enabled bridge", bridge, err)
	}
}

func lifecycleBridge(t *testing.T, endpoint string) *Bridge {
	t.Helper()
	t.Setenv("DISCORD_CHAT_SHARED_SECRET", strings.Repeat("s", 32))
	t.Setenv("DISCORD_CHAT_WEBHOOK_URL", endpoint)
	b, err := NewFromEnvironment(func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

func TestCloseBeforeStartAndRepeatedStart(t *testing.T) {
	b := lifecycleBridge(t, "http://127.0.0.1:1")
	done := make(chan struct{})
	go func() { b.Close(); b.Start(); b.Start(); b.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close before start did not join worker")
	}
	b.Enqueue(Message{SenderName: "test", Text: "after close"})
	if len(b.outbound) != 0 {
		t.Fatal("closed bridge accepted outbound work")
	}
}

func TestCloseCancelsInFlightDeliveryAndJoinsWorker(t *testing.T) {
	entered, cancelled := make(chan struct{}), make(chan struct{})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	}))
	t.Cleanup(endpoint.Close)
	b := lifecycleBridge(t, endpoint.URL)
	b.Start()
	b.Start()
	b.Enqueue(Message{SenderName: "test", Text: "local fixture"})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("delivery not started")
	}
	done := make(chan struct{})
	go func() { b.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not cancel delivery")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("HTTP delivery was not cancelled")
	}
}

type testRoundTrip func(*http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type retryResponseBody struct {
	io.Reader
	close func()
}

func (b retryResponseBody) Close() error { b.close(); return nil }

func TestCloseCancelsRetryDelayWithoutAnotherAttempt(t *testing.T) {
	b := lifecycleBridge(t, "http://local-fixture.invalid")
	responseClosed := make(chan struct{})
	var once sync.Once
	attempts := 0
	b.client.Transport = testRoundTrip(func(*http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 fixture", Body: retryResponseBody{strings.NewReader("retry"), func() { once.Do(func() { close(responseClosed) }) }}}, nil
	})
	b.Start()
	b.Enqueue(Message{SenderName: "test", Text: "retry fixture"})
	<-responseClosed
	done := make(chan struct{})
	go func() { b.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("close waited through retry delay")
	}
	if attempts != 1 {
		t.Fatalf("shutdown retried %d times", attempts)
	}
}
