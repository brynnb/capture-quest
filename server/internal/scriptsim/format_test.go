package scriptsim

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInventoryGoldenAliasesPreserveIdentityWithoutChangingEvidence(t *testing.T) {
	first := json.RawMessage(`{"instanceId":91,"inventory":{"commandRevision":1,"items":[{"instance":{"id":91,"quantity":1}},{"instance":{"id":92,"quantity":2}}]}}`)
	second := json.RawMessage(strings.ReplaceAll(strings.ReplaceAll(string(first), "91", "301"), "92", "302"))
	formatted := formatInventoryMessage(first)
	if formatted != formatInventoryMessage(second) || strings.Count(formatted, `"instance-1"`) != 2 || !strings.Contains(formatted, `"instance-2"`) || !strings.Contains(string(first), `"id":91`) {
		t.Fatalf("unstable or altered identity evidence: %s / %s", formatted, first)
	}
}
