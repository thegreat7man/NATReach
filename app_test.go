package main

import "testing"

func TestRandomPassword(t *testing.T) {
	a, b := randomPassword(), randomPassword()
	if len(a) < 20 || a == b {
		t.Fatalf("weak generated passwords: %q %q", a, b)
	}
}

func TestShellCommand(t *testing.T) {
	if _, err := shellCommand("", false); err != nil {
		t.Fatal(err)
	}
}
