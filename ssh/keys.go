package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var keyTypes = []string{"id_ed25519", "id_rsa", "id_ecdsa"}

// FindExistingPublicKey checks for known public key files in the given
// directory. Returns the path to the first one found, or an empty string
// if none exist.
func FindExistingPublicKey(sshDir string) (string, error) {
	for _, name := range keyTypes {
		pubPath := filepath.Join(sshDir, name+".pub")
		if _, err := os.Stat(pubPath); err == nil {
			return pubPath, nil
		}
	}
	return "", nil
}

// GenerateKey runs ssh-keygen to create an ed25519 key pair in the given
// directory. It returns the path to the public key file.
func GenerateKey(sshDir string) (string, error) {
	keyPath := filepath.Join(sshDir, "id_ed25519")
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh-keygen failed: %w", err)
	}
	return keyPath + ".pub", nil
}

// CopyKeyToHost copies the public key to the remote host. It tries
// ssh-copy-id first; if that binary is not available or fails, it falls
// back to a manual cat|ssh pipeline.
func CopyKeyToHost(pubKeyPath, user, address string, port int) error {
	// Try ssh-copy-id first.
	if copyIDPath, err := exec.LookPath("ssh-copy-id"); err == nil {
		args := []string{}
		if port != 0 {
			args = append(args, "-p", strconv.Itoa(port))
		}
		args = append(args, "-i", pubKeyPath, user+"@"+address)

		cmd := exec.Command(copyIDPath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Fallback: read the public key and pipe it via ssh.
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	args := []string{}
	if port != 0 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, user+"@"+address,
		"umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys")

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = strings.NewReader(string(pubKey))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh key copy fallback failed: %w", err)
	}
	return nil
}
