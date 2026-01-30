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

func TestFindExistingKeyPriority(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both RSA and ed25519 keys
	rsaPub := filepath.Join(tmpDir, "id_rsa.pub")
	ed25519Pub := filepath.Join(tmpDir, "id_ed25519.pub")

	if err := os.WriteFile(rsaPub, []byte("ssh-rsa AAAA..."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ed25519Pub, []byte("ssh-ed25519 AAAA..."), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindExistingPublicKey(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ed25519 should be preferred over RSA (first in keyTypes)
	if got != ed25519Pub {
		t.Errorf("expected ed25519 key %s, got %s", ed25519Pub, got)
	}
}

func TestFindExistingKeyRSAOnly(t *testing.T) {
	tmpDir := t.TempDir()

	rsaPub := filepath.Join(tmpDir, "id_rsa.pub")
	if err := os.WriteFile(rsaPub, []byte("ssh-rsa AAAA..."), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindExistingPublicKey(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rsaPub {
		t.Errorf("expected RSA key %s, got %s", rsaPub, got)
	}
}
