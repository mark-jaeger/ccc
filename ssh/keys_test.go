package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExistingKey(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake ed25519 key pair.
	privPath := filepath.Join(tmpDir, "id_ed25519")
	pubPath := privPath + ".pub"

	if err := os.WriteFile(privPath, []byte("fake-private-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAA... user@host"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindExistingPublicKey(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != pubPath {
		t.Errorf("expected %s, got %s", pubPath, got)
	}
}

func TestFindExistingKeyMissing(t *testing.T) {
	tmpDir := t.TempDir()

	got, err := FindExistingPublicKey(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing keys, got %s", got)
	}
}

func TestBuildCopyKeyCommand(t *testing.T) {
	cmd := buildCopyKeyFallbackCommand("/home/user/.ssh/id_ed25519.pub", "deploy", "10.0.0.1")
	if cmd == "" {
		t.Error("expected non-empty command string")
	}
}
