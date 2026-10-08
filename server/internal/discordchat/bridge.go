package discordchat

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxRequestBytes = 4 * 1024

type PublishFunc func(context.Context, string, string) error

type Message struct {
	SenderName string `json:"senderName"`
	Text       string `json:"text"`
}

type Bridge struct {
	secret     string
	webhookURL string
	publish    PublishFunc
	client     *http.Client
	outbound   chan Message
	context    context.Context
	cancel     context.CancelFunc
	start      sync.Once
	done       chan struct{}
	close      sync.Once
}

func NewFromEnvironment(publish PublishFunc) (*Bridge, error) {
	secret := strings.TrimSpace(os.Getenv("DISCORD_CHAT_SHARED_SECRET"))
	webhookURL := strings.TrimSpace(os.Getenv("DISCORD_CHAT_WEBHOOK_URL"))
	if secret == "" && webhookURL == "" {
		return nil, nil
	}
	if secret == "" || webhookURL == "" {
		return nil, errors.New("DISCORD_CHAT_SHARED_SECRET and DISCORD_CHAT_WEBHOOK_URL must both be configured")
	}
	if len(secret) < 32 {
		return nil, errors.New("DISCORD_CHAT_SHARED_SECRET must contain at least 32 characters")
	}
	if publish == nil {
		return nil, errors.New("Discord chat publisher is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Bridge{
		secret: secret, webhookURL: webhookURL, publish: publish,
		client:   &http.Client{Timeout: 8 * time.Second},
		outbound: make(chan Message, 256), context: ctx, cancel: cancel, done: make(chan struct{}),
	}, nil
}

func (b *Bridge) Start() { b.start.Do(func() { go b.run() }) }

func (b *Bridge) Close() {
	b.close.Do(func() {
		b.cancel()
		// Starting the cancelled worker also joins close-before-start safely.
		b.Start()
		<-b.done
	})
}

func (b *Bridge) Handler() http.Handler { return http.HandlerFunc(b.handle) }

func (b *Bridge) Enqueue(message Message) {
	if b.context.Err() != nil {
		return
	}
	select {
	case <-b.context.Done():
		return
	case b.outbound <- message:
	default:
		log.Printf("[DiscordChat] outbound queue full; dropped message from %s", message.SenderName)
	}
}

func (b *Bridge) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil || !b.verify(r.Header, body, time.Now()) {
		http.Error(w, "Invalid request", http.StatusUnauthorized)
		return
	}
	var message Message
	if err := json.Unmarshal(body, &message); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	message.SenderName = strings.TrimSpace(message.SenderName)
	message.Text = strings.TrimSpace(message.Text)
	if message.SenderName == "" || message.Text == "" {
		http.Error(w, "Sender and message are required", http.StatusBadRequest)
		return
	}
	if err := b.publish(r.Context(), message.SenderName, message.Text); err != nil {
		log.Printf("[DiscordChat] rejected inbound message: %v", err)
		http.Error(w, "Could not publish message", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (b *Bridge) verify(headers http.Header, body []byte, now time.Time) bool {
	timestampText := headers.Get("X-Game-Chat-Timestamp")
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || now.Sub(time.Unix(timestamp, 0)) > 5*time.Minute || time.Unix(timestamp, 0).Sub(now) > 5*time.Minute {
		return false
	}
	provided, err := hex.DecodeString(headers.Get("X-Game-Chat-Signature"))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(b.secret))
	_, _ = mac.Write([]byte(timestampText))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func (b *Bridge) run() {
	defer close(b.done)
	for {
		select {
		case <-b.context.Done():
			return
		case message := <-b.outbound:
			if b.context.Err() != nil {
				return
			}
			if err := b.deliver(message); err != nil && b.context.Err() == nil {
				log.Printf("[DiscordChat] webhook delivery failed for %s: %v", message.SenderName, err)
			}
		}
	}
}

func (b *Bridge) deliver(message Message) error {
	payloadBody := struct {
		Content         string `json:"content"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}{Content: fmt.Sprintf("[GAMECHAT] %s: %s", message.SenderName, message.Text)}
	payloadBody.AllowedMentions.Parse = []string{}
	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := b.context.Err(); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(b.context, http.MethodPost, b.webhookURL+"?wait=true", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := b.client.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("Discord returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
		if response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
			return lastErr
		}
		delay := time.NewTimer(time.Duration(attempt+1) * 500 * time.Millisecond)
		select {
		case <-b.context.Done():
			delay.Stop()
			return b.context.Err()
		case <-delay.C:
		}
	}
	return lastErr
}
