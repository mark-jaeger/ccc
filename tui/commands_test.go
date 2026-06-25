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

	t.Run("no fallbacks: underlying primary error is preserved", func(t *testing.T) {
		host := config.Host{User: "me", Address: "primary.example"}
		test := func(c *ssh.Connection) error { return errors.New("host key verification failed") }
		_, err := selectWorkingConnection(host, test)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "host key verification failed") {
			t.Errorf("error should preserve the underlying SSH failure, got: %s", err.Error())
		}
	})

	t.Run("all fail: each underlying error is preserved", func(t *testing.T) {
		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example"},
		}
		test := func(c *ssh.Connection) error {
			if c.Address == "primary.example" {
				return errors.New("permission denied (publickey)")
			}
			return errors.New("connection timed out")
		}
		_, err := selectWorkingConnection(host, test)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, want := range []string{"permission denied (publickey)", "connection timed out"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should preserve %q, got: %s", want, err.Error())
			}
		}
	})
}

// TestConnectHostCmd covers the wiring around selectWorkingConnection: when a
// fallback is selected, the address that worked must reach the model on BOTH the
// runner and the emitted host. The model rebuilds attach/create connections from
// currentHost (= this host), so a stale primary address there would make session
// attach/create dial the dead primary even though probing succeeded over the
// fallback. The connectionTester seam lets us exercise this without real SSH.
func TestConnectHostCmd(t *testing.T) {
	orig := connectionTester
	defer func() { connectionTester = orig }()

	t.Run("selected fallback address propagates to host and runner", func(t *testing.T) {
		var attempts []string
		connectionTester = fakeTest("fallback1.example", &attempts)

		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example", "fallback2.example"},
		}
		msg := connectHostCmd("box", host)()

		hc, ok := msg.(hostConnectedMsg)
		if !ok {
			t.Fatalf("expected hostConnectedMsg, got %T (%v)", msg, msg)
		}
		if hc.hostName != "box" {
			t.Errorf("hostName = %q, want box", hc.hostName)
		}
		if hc.host.Address != "fallback1.example" {
			t.Errorf("emitted host.Address = %q, want fallback1.example (attach/create rebuild from this)", hc.host.Address)
		}
		conn, ok := hc.runner.(*ssh.Connection)
		if !ok {
			t.Fatalf("runner is %T, want *ssh.Connection", hc.runner)
		}
		if conn.Address != "fallback1.example" {
			t.Errorf("runner address = %q, want fallback1.example", conn.Address)
		}
	})

	t.Run("primary works, host address unchanged", func(t *testing.T) {
		var attempts []string
		connectionTester = fakeTest("primary.example", &attempts)

		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example"},
		}
		msg := connectHostCmd("box", host)()

		hc, ok := msg.(hostConnectedMsg)
		if !ok {
			t.Fatalf("expected hostConnectedMsg, got %T (%v)", msg, msg)
		}
		if hc.host.Address != "primary.example" {
			t.Errorf("emitted host.Address = %q, want primary.example", hc.host.Address)
		}
	})

	t.Run("all addresses fail yields errMsg", func(t *testing.T) {
		var attempts []string
		connectionTester = fakeTest("", &attempts)

		host := config.Host{
			User:              "me",
			Address:           "primary.example",
			FallbackAddresses: []string{"fallback1.example"},
		}
		msg := connectHostCmd("box", host)()
		if _, ok := msg.(errMsg); !ok {
			t.Fatalf("expected errMsg, got %T (%v)", msg, msg)
		}
	})
}
