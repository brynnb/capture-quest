package world

import (
	"log"
	"strings"
	"sync"
	"unicode"

	"capturequest/internal/db"
)

// chatFilter holds the loaded disallowed words and provides censoring.
// Words are stored in their normalized form (leet-speak decoded, lowercased).
// At match time, the input message is also normalized before scanning.
var chatFilter struct {
	mu    sync.RWMutex
	words []string // normalized canonical forms
}

// leetMap maps common obfuscation characters to their alphabetic equivalent.
var leetMap = map[rune]rune{
	'0': 'o',
	'1': 'i',
	'3': 'e',
	'4': 'a',
	'5': 's',
	'7': 't',
	'8': 'b',
	'9': 'g',
	'@': 'a',
	'$': 's',
	'!': 'i',
	'¡': 'i',
	'+': 't',
	'(': 'c',
	'|': 'l',
}

// normalizeText converts a string to a canonical form for matching:
// - lowercased
// - leet-speak characters decoded
// - non-alphanumeric/space characters stripped
func normalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if mapped, ok := leetMap[r]; ok {
			b.WriteRune(mapped)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
		// else: strip the character (*, -, _, etc.)
	}
	return b.String()
}

// LoadDisallowedWords reads all words from the disallowed_words table
// and stores their normalized forms for matching.
func LoadDisallowedWords() {
	myDB := db.GlobalWorldDB.DB
	if myDB == nil {
		log.Println("[ChatFilter] No DB connection, skipping disallowed words load")
		return
	}

	rows, err := myDB.Query("SELECT word FROM disallowed_words")
	if err != nil {
		log.Printf("[ChatFilter] Failed to load disallowed words: %v", err)
		return
	}
	defer rows.Close()

	var words []string
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			continue
		}
		norm := normalizeText(w)
		if norm != "" {
			words = append(words, norm)
		}
	}

	chatFilter.mu.Lock()
	chatFilter.words = words
	chatFilter.mu.Unlock()

	log.Printf("[ChatFilter] Loaded %d disallowed words", len(words))
}

// CensorMessage replaces any disallowed words found in the message with
// asterisks of the same length as the matched substring in the original text.
// Matching is done on the normalized (leet-decoded) form, but replacement
// is applied to the original text at the corresponding positions.
func CensorMessage(message string) string {
	chatFilter.mu.RLock()
	words := chatFilter.words
	chatFilter.mu.RUnlock()

	if len(words) == 0 {
		return message
	}

	// Build a normalized version and a mapping from normalized index → original index.
	// Because normalization can remove characters, we need to track which original
	// characters map to which normalized positions.
	type indexPair struct {
		origIdx int
		origLen int // byte length of the original rune
	}

	runes := []rune(message)
	var normBuilder strings.Builder
	normBuilder.Grow(len(message))
	var mapping []indexPair // one entry per rune in the normalized string

	origBytePos := 0
	for _, r := range runes {
		runeByteLen := len(string(r))
		lower := unicode.ToLower(r)

		if mapped, ok := leetMap[lower]; ok {
			normBuilder.WriteRune(mapped)
			mapping = append(mapping, indexPair{origBytePos, runeByteLen})
		} else if unicode.IsLetter(lower) || unicode.IsDigit(lower) || lower == ' ' {
			normBuilder.WriteRune(lower)
			mapping = append(mapping, indexPair{origBytePos, runeByteLen})
		}
		// else: character is stripped from normalized form, no mapping entry

		origBytePos += runeByteLen
	}

	normalized := normBuilder.String()
	normRunes := []rune(normalized)

	// For each disallowed word, scan the normalized text for occurrences.
	// Mark the corresponding original byte ranges for censoring.
	type censorRange struct {
		start, end int // byte positions in the original string
	}
	var ranges []censorRange

	for _, word := range words {
		wordRunes := []rune(word)
		wLen := len(wordRunes)
		if wLen == 0 {
			continue
		}

		for i := 0; i <= len(normRunes)-wLen; i++ {
			// Check for word boundary: the match shouldn't be in the middle of a larger word.
			// Allow match at start of string or after a space/non-letter.
			if i > 0 && isWordChar(normRunes[i-1]) {
				continue
			}
			// Allow match at end of string or before a space/non-letter.
			endIdx := i + wLen
			if endIdx < len(normRunes) && isWordChar(normRunes[endIdx]) {
				continue
			}

			// Check if the substring matches
			match := true
			for j := 0; j < wLen; j++ {
				if normRunes[i+j] != wordRunes[j] {
					match = false
					break
				}
			}
			if match {
				// Map back to original byte positions
				origStart := mapping[i].origIdx
				lastMapping := mapping[i+wLen-1]
				origEnd := lastMapping.origIdx + lastMapping.origLen
				ranges = append(ranges, censorRange{origStart, origEnd})
			}
		}
	}

	if len(ranges) == 0 {
		return message
	}

	// Apply censoring: replace matched byte ranges with asterisks
	result := []byte(message)
	for _, r := range ranges {
		for i := r.start; i < r.end && i < len(result); i++ {
			// Only replace non-space bytes to preserve word spacing
			if result[i] != ' ' {
				result[i] = '*'
			}
		}
	}

	return string(result)
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
