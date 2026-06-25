package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/ssh"
)

// fakeTest builds a test func for selectWorkingConnection that succeeds only
// for the address in okAddr. It records every attempted address in *attempts so
// callers can assert order. An empty okAddr makes every address fail.
func fakeTest(okAddr string, attempts *[]string) func(*ssh.Connection) error {
	return func(c *ssh.Connection) error {
		*attempts = append(*attempts, c.Address)
		if okAddr != "" && c.Address == okAddr {
			return nil
		}
		return errors.New("unreachable")
	}
}

func TestSelectWorkingConnection(t *testing.T) {
	t.Run("primary works, no fallback tried", func(t *testing.T) {
		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example", "fallback2.example"},
		}
		var attempts []string
		conn, err := selectWorkingConnection(host, fakeTest("primary.example", &attempts))
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if conn.Address != "primary.example" {
			t.Errorf("expected primary address, got %q", conn.Address)
		}
		if len(attempts) != 1 || attempts[0] != "primary.example" {
			t.Errorf("expected only the primary to be tried, got %v", attempts)
		}
	})

	t.Run("primary fails, second fallback works", func(t *testing.T) {
		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example", "fallback2.example"},
		}
		var attempts []string
		conn, err := selectWorkingConnection(host, fakeTest("fallback2.example", &attempts))
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if conn.Address != "fallback2.example" {
			t.Errorf("expected fallback2 address, got %q", conn.Address)
		}
		want := []string{"primary.example", "fallback1.example", "fallback2.example"}
		if len(attempts) != len(want) {
			t.Fatalf("expected attempts %v, got %v", want, attempts)
		}
		for i := range want {
			if attempts[i] != want[i] {
				t.Errorf("attempt %d: expected %q, got %q", i, want[i], attempts[i])
			}
		}
	})

	t.Run("primary and all fallbacks fail", func(t *testing.T) {
		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example", "fallback2.example"},
		}
		var attempts []string
		conn, err := selectWorkingConnection(host, fakeTest("", &attempts))
		if err == nil {
			t.Fatalf("expected error, got connection %+v", conn)
		}
		msg := err.Error()
		for _, addr := range []string{"primary.example", "fallback1.example", "fallback2.example"} {
			if !strings.Contains(msg, addr) {
				t.Errorf("error should mention %q, got: %s", addr, msg)
			}
		}
	})

	t.Run("no fallbacks configured and primary fails", func(t *testing.T) {
		host := config.Host{
			User:    "me",
			Address: "primary.example",
		}
		var attempts []string
		conn, err := selectWorkingConnection(host, fakeTest("", &attempts))
		if err == nil {
			t.Fatalf("expected error, got connection %+v", conn)
		}
		if len(attempts) != 1 || attempts[0] != "primary.example" {
			t.Errorf("expected only the primary to be tried, got %v", attempts)
		}
		if !strings.Contains(err.Error(), "primary.example") {
			t.Errorf("error should mention the primary address, got: %s", err.Error())
		}
	})
}
