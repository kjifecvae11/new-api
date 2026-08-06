package main

import (
	"net/http"
	"testing"
)

func TestSessionCookieSecureByDefault(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "")
	options := sessionCookieOptions()
	if !options.Secure {
		t.Fatal("Secure = false, want true by default")
	}
	if !options.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if options.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want Strict", options.SameSite)
	}
}

func TestSessionCookieCanBeExplicitlyInsecureForLocalHTTP(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	if sessionCookieOptions().Secure {
		t.Fatal("Secure = true, want false for explicit local HTTP override")
	}
}
