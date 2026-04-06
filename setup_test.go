package main

import (
	"strings"
	"testing"
)

func TestPAMRuleShowsFriendlyOutput(t *testing.T) {
	if !strings.Contains(pamRule, " quiet ") {
		t.Fatalf("pamRule = %q, want quiet option", pamRule)
	}
	if !strings.Contains(pamRule, " stdout ") {
		t.Fatalf("pamRule = %q, want stdout option", pamRule)
	}
}
