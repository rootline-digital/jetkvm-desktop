package main

import "testing"

func TestResolvePassword(t *testing.T) {
	getenv := func(name string) string {
		switch name {
		case defaultPasswordEnv:
			return "default-secret"
		case "CUSTOM_PASSWORD":
			return "custom-secret"
		default:
			return ""
		}
	}

	t.Run("explicit env var wins", func(t *testing.T) {
		if got := resolvePassword("CUSTOM_PASSWORD", getenv); got != "custom-secret" {
			t.Fatalf("resolvePassword(CUSTOM_PASSWORD) = %q, want custom-secret", got)
		}
	})

	t.Run("default env fallback", func(t *testing.T) {
		if got := resolvePassword("", getenv); got != "default-secret" {
			t.Fatalf("resolvePassword(\"\") = %q, want default-secret", got)
		}
	})

	t.Run("unknown env var resolves empty", func(t *testing.T) {
		if got := resolvePassword("MISSING_PASSWORD", getenv); got != "" {
			t.Fatalf("resolvePassword(MISSING_PASSWORD) = %q, want empty", got)
		}
	})
}
