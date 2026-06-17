package world

import "testing"

func TestCensorMessageMatchesObfuscatedWords(t *testing.T) {
	chatFilter.mu.Lock()
	previousWords := chatFilter.words
	chatFilter.words = []string{normalizeText("badword")}
	chatFilter.mu.Unlock()
	t.Cleanup(func() {
		chatFilter.mu.Lock()
		chatFilter.words = previousWords
		chatFilter.mu.Unlock()
	})

	got := CensorMessage("hello b@dw0rd friend")
	if got != "hello ******* friend" {
		t.Fatalf("CensorMessage() = %q, want obfuscated word censored", got)
	}
}
