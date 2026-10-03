package scriptsim

import (
	"strings"
	"testing"
)

func TestSafariTerminalPresenceIsDistinctFromPlayableEncounter(t *testing.T) {
	no := false
	terminal := &SafariSummary{Active: false, Battle: &SafariBattleSummary{PokemonID: 111, Name: "RHYHORN", Level: 25, IsOver: true}}
	expected := SafariBattleExpected{Active: &no, PokemonID: 111, Level: 25}
	if err := validateSafariBattleExpectation(terminal, expected); err != nil {
		t.Fatal(err)
	}
	// Retention is required: changing the expectation to inactive must not hide
	// deletion of the encounter or accidentally accept an unfinished encounter.
	if err := validateSafariBattleExpectation(&SafariSummary{}, expected); err == nil {
		t.Fatal("missing terminal encounter accepted")
	}
	terminal.Battle.IsOver = false
	if err := validateSafariBattleExpectation(terminal, expected); err == nil {
		t.Fatal("playable encounter accepted as terminal")
	}
	terminal.Battle.IsOver = true
	var output strings.Builder
	writeSafariSummary(&output, terminal)
	if !strings.Contains(output.String(), "inactive") || !strings.Contains(output.String(), "battle: #111 RHYHORN L25 caught=false fled=false over=true") {
		t.Fatal("inactive retained encounter hidden", output.String())
	}
}
