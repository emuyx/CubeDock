package main

import "testing"

func TestSafeLogStripsControlCharacters(t *testing.T) {
	got := safeLog("1.0.0\nFORGED\t\x00\r")
	if got != "1.0.0FORGED" {
		t.Fatalf("safeLog = %q", got)
	}
	if got := safeLog("1.21.130.0"); got != "1.21.130.0" {
		t.Fatalf("normal value changed: %q", got)
	}
}
