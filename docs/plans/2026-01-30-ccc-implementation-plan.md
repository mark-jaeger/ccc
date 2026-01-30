# ccc Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that connects to remote hosts via SSH and manages tmux sessions per project, hiding all ceremony behind numbered menus.

**Architecture:** Single binary, no dependencies beyond a TOML parser. Shells out to `ssh`, `ssh-keygen`, `tmux`, `tailscale` (optional). Two TOML config files: client-side (`~/.ccc/config.toml`) for hosts, host-side (`~/.ccc/projects.toml`) for projects. Remote operations use non-interactive SSH with `BatchMode=yes`; attachment uses interactive SSH with `-t`. Local mode auto-detected via `$SSH_CONNECTION`.

**Tech Stack:** Go 1.25, `pelletier/go-toml/v2`, Go stdlib (`os/exec`, `bufio`, `fmt`, `strings`, `path/filepath`)

**Design doc:** `docs/plans/2026-01-30-ccc-cli-design.md`

---

## Task 1: Go Module + Project Skeleton

**Files:**
- Create: `go.mod`
- Create: `main.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/markjd/ccc`

**Step 2: Create main.go with argument routing**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "local" {
		fmt.Println("local mode: not yet implemented")
		os.Exit(0)
	}

	fmt.Println("ccc: not yet implemented")
}
```

**Step 3: Verify it builds and runs**

Run: `go build -o ccc . && ./ccc`
Expected: prints "ccc: not yet implemented"

Run: `./ccc local`
Expected: prints "local mode: not yet implemented"

**Step 4: Commit**

```bash
git add go.mod main.go
git commit -m "feat: initialize Go module and main entry point"
```

---

## Task 2: UI — Menu System

The menu is used everywhere. Build and test it first.

**Files:**
- Create: `ui/menu.go`
- Create: `ui/menu_test.go`

**Step 1: Write failing tests for menu**

```go
package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestMenuSelectByNumber(t *testing.T) {
	in := strings.NewReader("2\n")
	out := &bytes.Buffer{}

	items := []MenuItem{
		{Key: "rt1", Label: "rt1"},
		{Key: "pro-rag", Label: "pro-rag"},
	}
	result, err := ShowMenu(in, out, MenuConfig{
		Title: "Projects",
		Items: items,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionSelect || result.Selected.Key != "pro-rag" {
		t.Errorf("expected pro-rag, got %+v", result)
	}
}

func TestMenuQuit(t *testing.T) {
	in := strings.NewReader("q\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title: "Hosts",
		Items: []MenuItem{{Key: "h1", Label: "host1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionQuit {
		t.Errorf("expected quit, got %+v", result)
	}
}

func TestMenuBack(t *testing.T) {
	in := strings.NewReader("b\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title:    "Sessions",
		Items:    []MenuItem{{Key: "s1", Label: "session1"}},
		ShowBack: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionBack {
		t.Errorf("expected back, got %+v", result)
	}
}

func TestMenuRemove(t *testing.T) {
	// User types 'r', then '1', then 'y' to confirm
	in := strings.NewReader("r\n1\ny\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title:      "Sessions",
		Items:      []MenuItem{{Key: "s1", Label: "session1"}},
		ShowRemove: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionRemove || result.Selected.Key != "s1" {
		t.Errorf("expected remove s1, got %+v", result)
	}
}

func TestMenuExtraActions(t *testing.T) {
	in := strings.NewReader("a\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title: "Hosts",
		Items: []MenuItem{{Key: "h1", Label: "host1"}},
		ExtraActions: []ExtraAction{
			{Key: "a", Label: "Add host", Action: "add"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionExtra || result.ExtraKey != "add" {
		t.Errorf("expected extra add, got %+v", result)
	}
}

func TestMenuInvalidThenValid(t *testing.T) {
	in := strings.NewReader("99\n1\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title: "Hosts",
		Items: []MenuItem{{Key: "h1", Label: "host1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionSelect || result.Selected.Key != "h1" {
		t.Errorf("expected h1, got %+v", result)
	}
	if !strings.Contains(out.String(), "Invalid") {
		t.Error("expected invalid input message")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -v`
Expected: FAIL — package/types don't exist yet.

**Step 3: Implement menu**

```go
package ui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Action int

const (
	ActionSelect Action = iota
	ActionQuit
	ActionBack
	ActionRemove
	ActionExtra
)

type MenuItem struct {
	Key   string
	Label string
	Extra string // optional extra info shown after label
}

type ExtraAction struct {
	Key    string // single char like "a", "s", "n"
	Label  string // display text
	Action string // returned in result
}

type MenuConfig struct {
	Title        string
	Items        []MenuItem
	Prompt       string // defaults to "Select"
	ShowBack     bool
	ShowRemove   bool
	ExtraActions []ExtraAction
}

type MenuResult struct {
	Action   Action
	Selected MenuItem
	ExtraKey string // which extra action was chosen
}

func ShowMenu(in io.Reader, out io.Writer, cfg MenuConfig) (MenuResult, error) {
	scanner := bufio.NewScanner(in)
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = "Select"
	}

	for {
		fmt.Fprintf(out, "\n  %s\n", cfg.Title)
		for i, item := range cfg.Items {
			if item.Extra != "" {
				fmt.Fprintf(out, "  [%d] %s %s\n", i+1, item.Label, item.Extra)
			} else {
				fmt.Fprintf(out, "  [%d] %s\n", i+1, item.Label)
			}
		}
		for _, ea := range cfg.ExtraActions {
			fmt.Fprintf(out, "  [%s] %s\n", ea.Key, ea.Label)
		}
		if cfg.ShowBack {
			fmt.Fprintf(out, "  [b] Back\n")
		}
		fmt.Fprintf(out, "  [q] Quit\n")

		if cfg.ShowRemove {
			fmt.Fprintf(out, "\n  %s (or 'r' to remove): ", prompt)
		} else {
			fmt.Fprintf(out, "\n  %s: ", prompt)
		}

		if !scanner.Scan() {
			return MenuResult{Action: ActionQuit}, scanner.Err()
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "q" {
			return MenuResult{Action: ActionQuit}, nil
		}
		if input == "b" && cfg.ShowBack {
			return MenuResult{Action: ActionBack}, nil
		}
		if input == "r" && cfg.ShowRemove {
			return handleRemove(scanner, out, cfg)
		}
		for _, ea := range cfg.ExtraActions {
			if input == ea.Key {
				return MenuResult{Action: ActionExtra, ExtraKey: ea.Action}, nil
			}
		}

		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(cfg.Items) {
			fmt.Fprintf(out, "  Invalid selection. Try again.\n")
			continue
		}
		return MenuResult{Action: ActionSelect, Selected: cfg.Items[num-1]}, nil
	}
}

func handleRemove(scanner *bufio.Scanner, out io.Writer, cfg MenuConfig) (MenuResult, error) {
	fmt.Fprintf(out, "  Select item to remove: ")
	if !scanner.Scan() {
		return MenuResult{Action: ActionQuit}, scanner.Err()
	}
	input := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(cfg.Items) {
		fmt.Fprintf(out, "  Invalid selection.\n")
		return MenuResult{Action: ActionQuit}, nil
	}
	item := cfg.Items[num-1]

	fmt.Fprintf(out, "  Remove %s? (y/n): ", item.Label)
	if !scanner.Scan() {
		return MenuResult{Action: ActionQuit}, scanner.Err()
	}
	confirm := strings.TrimSpace(scanner.Text())
	if confirm != "y" && confirm != "Y" {
		return MenuResult{Action: ActionQuit}, nil
	}
	return MenuResult{Action: ActionRemove, Selected: item}, nil
}

// Prompt asks a simple question and returns the answer.
func Prompt(in io.Reader, out io.Writer, question string) (string, error) {
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "  %s: ", question)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// Confirm asks a y/n question.
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "  %s (y/n): ", question)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.TrimSpace(scanner.Text())
	return answer == "y" || answer == "Y", nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./ui/ -v`
Expected: all 6 tests PASS.

**Step 5: Commit**

```bash
git add ui/
git commit -m "feat: add interactive menu system with numbered selection"
```

---

## Task 3: Config — Client Config (Read/Write)

**Files:**
- Create: `config/client.go`
- Create: `config/client_test.go`

**Step 1: Install TOML dependency**

Run: `go get github.com/pelletier/go-toml/v2`

**Step 2: Write failing tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(`
[hosts.macbook-m1]
user = "mark"
address = "100.64.0.1"

[hosts.server-lab]
user = "mark"
address = "100.64.0.5"
`), 0644)

	cfg, err := LoadClientConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	h := cfg.Hosts["macbook-m1"]
	if h.User != "mark" || h.Address != "100.64.0.1" {
		t.Errorf("unexpected host: %+v", h)
	}
}

func TestLoadClientConfigMissing(t *testing.T) {
	_, err := LoadClientConfig("/nonexistent/config.toml")
	if err != ErrNoConfig {
		t.Errorf("expected ErrNoConfig, got %v", err)
	}
}

func TestSaveClientConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &ClientConfig{
		Hosts: map[string]Host{
			"test-host": {User: "admin", Address: "10.0.0.1"},
		},
	}
	err := SaveClientConfig(path, cfg)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadClientConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Hosts["test-host"].User != "admin" {
		t.Errorf("round-trip failed: %+v", loaded)
	}
}

func TestAddHost(t *testing.T) {
	cfg := &ClientConfig{Hosts: map[string]Host{}}
	cfg.AddHost("new-host", Host{User: "u", Address: "1.2.3.4"})
	if _, ok := cfg.Hosts["new-host"]; !ok {
		t.Error("host not added")
	}
}

func TestRemoveHost(t *testing.T) {
	cfg := &ClientConfig{Hosts: map[string]Host{
		"h1": {User: "u", Address: "1.2.3.4"},
	}}
	cfg.RemoveHost("h1")
	if _, ok := cfg.Hosts["h1"]; ok {
		t.Error("host not removed")
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./config/ -v`
Expected: FAIL.

**Step 4: Implement client config**

```go
package config

import (
	"errors"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

var ErrNoConfig = errors.New("config file not found")

type Host struct {
	User         string   `toml:"user"`
	Address      string   `toml:"address"`
	Port         int      `toml:"port,omitempty"`
	IdentityFile string   `toml:"identity_file,omitempty"`
	ProxyJump    string   `toml:"proxy_jump,omitempty"`
	SSHOptions   []string `toml:"ssh_options,omitempty"`
}

type ClientConfig struct {
	Hosts map[string]Host `toml:"hosts"`
}

func DefaultClientConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ccc", "config.toml")
}

func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoConfig
		}
		return nil, err
	}
	var cfg ClientConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]Host)
	}
	return &cfg, nil
}

func SaveClientConfig(path string, cfg *ClientConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *ClientConfig) AddHost(name string, host Host) {
	if c.Hosts == nil {
		c.Hosts = make(map[string]Host)
	}
	c.Hosts[name] = host
}

func (c *ClientConfig) RemoveHost(name string) {
	delete(c.Hosts, name)
}

// SortedHostNames returns host names in sorted order for stable display.
func (c *ClientConfig) SortedHostNames() []string {
	names := make([]string, 0, len(c.Hosts))
	for name := range c.Hosts {
		names = append(names, name)
	}
	// sort alphabetically
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./config/ -v`
Expected: all 5 tests PASS.

**Step 6: Commit**

```bash
git add config/ go.mod go.sum
git commit -m "feat: add client config read/write with TOML"
```

---

## Task 4: Config — Host/Projects Config (Parse Only)

Host-side config is read over SSH (or locally). This task handles parsing only — SSH transport comes later.

**Files:**
- Create: `config/projects.go`
- Create: `config/projects_test.go`

**Step 1: Write failing tests**

```go
package config

import (
	"testing"
)

func TestParseProjectsConfig(t *testing.T) {
	data := `
[projects.rt1]
path = "/Users/mark/Projects/jd/rt1"

[projects.death-and-taxes]
path = "/Users/mark/Projects/jd/death_and_taxes"
`
	cfg, err := ParseProjectsConfig([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects["rt1"].Path != "/Users/mark/Projects/jd/rt1" {
		t.Errorf("unexpected path: %s", cfg.Projects["rt1"].Path)
	}
}

func TestParseProjectsConfigEmpty(t *testing.T) {
	cfg, err := ParseProjectsConfig([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(cfg.Projects))
	}
}

func TestSerializeProjectsConfig(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: map[string]Project{
			"rt1": {Path: "/Users/mark/Projects/jd/rt1"},
		},
	}
	data, err := SerializeProjectsConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// round-trip
	cfg2, err := ParseProjectsConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Projects["rt1"].Path != "/Users/mark/Projects/jd/rt1" {
		t.Errorf("round-trip failed")
	}
}

func TestSortedProjectKeys(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: map[string]Project{
			"zebra": {Path: "/z"},
			"alpha": {Path: "/a"},
		},
	}
	keys := cfg.SortedProjectKeys()
	if keys[0] != "alpha" || keys[1] != "zebra" {
		t.Errorf("expected sorted, got %v", keys)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./config/ -v -run TestParse`
Expected: FAIL.

**Step 3: Implement projects config**

```go
package config

import (
	toml "github.com/pelletier/go-toml/v2"
)

type Project struct {
	Path string `toml:"path"`
}

type ProjectsConfig struct {
	Projects map[string]Project `toml:"projects"`
}

func ParseProjectsConfig(data []byte) (*ProjectsConfig, error) {
	var cfg ProjectsConfig
	if len(data) == 0 {
		cfg.Projects = make(map[string]Project)
		return &cfg, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Projects == nil {
		cfg.Projects = make(map[string]Project)
	}
	return &cfg, nil
}

func SerializeProjectsConfig(cfg *ProjectsConfig) ([]byte, error) {
	return toml.Marshal(cfg)
}

func (c *ProjectsConfig) SortedProjectKeys() []string {
	keys := make([]string, 0, len(c.Projects))
	for k := range c.Projects {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./config/ -v`
Expected: all tests PASS (both client and projects).

**Step 5: Commit**

```bash
git add config/projects.go config/projects_test.go
git commit -m "feat: add projects config parser and serializer"
```

---

## Task 5: SSH — Command Execution

**Files:**
- Create: `ssh/connection.go`
- Create: `ssh/connection_test.go`

Note: Tests for SSH are necessarily integration-flavored. We'll test command building (which is pure), and mark actual SSH execution as requiring a `CCC_TEST_HOST` env var to skip in CI.

**Step 1: Write failing tests**

```go
package ssh

import (
	"strings"
	"testing"
)

func TestBuildNonInteractiveArgs(t *testing.T) {
	c := &Connection{User: "mark", Address: "100.64.0.1"}
	args := c.buildNonInteractiveArgs("cat ~/.ccc/projects.toml")

	// Must contain BatchMode and StrictHostKeyChecking
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "BatchMode=yes") {
		t.Error("missing BatchMode=yes")
	}
	if !strings.Contains(joined, "StrictHostKeyChecking=accept-new") {
		t.Error("missing StrictHostKeyChecking=accept-new")
	}
	if !strings.Contains(joined, "bash -lc") {
		t.Error("missing login shell wrapper")
	}
	if !strings.Contains(joined, "mark@100.64.0.1") {
		t.Error("missing user@address")
	}
}

func TestBuildInteractiveArgs(t *testing.T) {
	c := &Connection{User: "mark", Address: "100.64.0.1"}
	args := c.buildInteractiveArgs("tmux attach -t rt1")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-t") {
		t.Error("missing -t for PTY")
	}
	// Should NOT contain BatchMode
	if strings.Contains(joined, "BatchMode") {
		t.Error("interactive should not use BatchMode")
	}
}

func TestBuildArgsWithPort(t *testing.T) {
	c := &Connection{User: "mark", Address: "100.64.0.1", Port: 2222}
	args := c.buildNonInteractiveArgs("echo hi")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-p 2222") && !strings.Contains(joined, "-p2222") {
		// check for -p flag
		found := false
		for i, a := range args {
			if a == "-p" && i+1 < len(args) && args[i+1] == "2222" {
				found = true
			}
		}
		if !found {
			t.Errorf("missing port flag in: %s", joined)
		}
	}
}

func TestBuildArgsWithIdentityFile(t *testing.T) {
	c := &Connection{User: "mark", Address: "100.64.0.1", IdentityFile: "~/.ssh/my_key"}
	args := c.buildNonInteractiveArgs("echo hi")
	found := false
	for i, a := range args {
		if a == "-i" && i+1 < len(args) && args[i+1] == "~/.ssh/my_key" {
			found = true
		}
	}
	if !found {
		t.Error("missing identity file flag")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./ssh/ -v`
Expected: FAIL.

**Step 3: Implement SSH connection**

```go
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Connection struct {
	User         string
	Address      string
	Port         int
	IdentityFile string
	ProxyJump    string
	SSHOptions   []string
}

func (c *Connection) target() string {
	return fmt.Sprintf("%s@%s", c.User, c.Address)
}

func (c *Connection) commonArgs() []string {
	var args []string
	if c.Port != 0 {
		args = append(args, "-p", strconv.Itoa(c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.ProxyJump != "" {
		args = append(args, "-J", c.ProxyJump)
	}
	for _, opt := range c.SSHOptions {
		args = append(args, opt)
	}
	return args
}

func (c *Connection) buildNonInteractiveArgs(cmd string) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	args = append(args, c.commonArgs()...)
	args = append(args, c.target())
	args = append(args, "bash", "-lc", fmt.Sprintf("%q", cmd))
	return args
}

func (c *Connection) buildInteractiveArgs(cmd string) []string {
	args := []string{"-t"}
	args = append(args, c.commonArgs()...)
	args = append(args, c.target(), cmd)
	return args
}

// RunCommand runs a non-interactive command over SSH and returns stdout.
func (c *Connection) RunCommand(cmd string) (string, error) {
	args := c.buildNonInteractiveArgs(cmd)
	out, err := exec.Command("ssh", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ssh command failed: %s\nstderr: %s", cmd, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("ssh command failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunInteractive hands over the terminal to an interactive SSH session.
func (c *Connection) RunInteractive(cmd string) error {
	args := c.buildInteractiveArgs(cmd)
	proc := exec.Command("ssh", args...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	return proc.Run()
}

// TestConnection verifies SSH access with a simple command.
func (c *Connection) TestConnection() error {
	_, err := c.RunCommand("echo ok")
	return err
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./ssh/ -v`
Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add ssh/
git commit -m "feat: add SSH command execution with BatchMode and login shell"
```

---

## Task 6: SSH — Key Setup

**Files:**
- Create: `ssh/keys.go`
- Create: `ssh/keys_test.go`

**Step 1: Write failing tests**

```go
package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExistingKey(t *testing.T) {
	dir := t.TempDir()
	// Create a fake key pair
	os.WriteFile(filepath.Join(dir, "id_ed25519"), []byte("private"), 0600)
	os.WriteFile(filepath.Join(dir, "id_ed25519.pub"), []byte("ssh-ed25519 AAAA..."), 0644)

	pub, err := FindExistingPublicKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pub == "" {
		t.Error("expected to find key")
	}
}

func TestFindExistingKeyMissing(t *testing.T) {
	dir := t.TempDir()
	pub, err := FindExistingPublicKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pub != "" {
		t.Error("expected no key")
	}
}

func TestBuildCopyKeyCommand(t *testing.T) {
	cmd := buildCopyKeyFallbackCommand("/home/user/.ssh/id_ed25519.pub", "mark", "10.0.0.1")
	if cmd == "" {
		t.Error("expected non-empty command")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./ssh/ -v -run TestFind`
Expected: FAIL.

**Step 3: Implement key utilities**

```go
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Key types to look for, in preference order.
var keyTypes = []string{"id_ed25519", "id_rsa", "id_ecdsa"}

// FindExistingPublicKey looks for known SSH public key files in sshDir.
func FindExistingPublicKey(sshDir string) (string, error) {
	for _, kt := range keyTypes {
		pubPath := filepath.Join(sshDir, kt+".pub")
		if _, err := os.Stat(pubPath); err == nil {
			return pubPath, nil
		}
	}
	return "", nil
}

// GenerateKey creates a new ed25519 SSH key pair.
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

// CopyKeyToHost installs a public key on the remote host.
// Tries ssh-copy-id first, falls back to manual append.
func CopyKeyToHost(pubKeyPath, user, address string, port int) error {
	// Try ssh-copy-id first
	if _, err := exec.LookPath("ssh-copy-id"); err == nil {
		args := []string{"-i", pubKeyPath}
		if port != 0 {
			args = append(args, "-p", fmt.Sprintf("%d", port))
		}
		args = append(args, fmt.Sprintf("%s@%s", user, address))
		cmd := exec.Command("ssh-copy-id", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fall through to manual method
	}

	// Fallback: manual append
	target := fmt.Sprintf("%s@%s", user, address)
	sshArgs := []string{}
	if port != 0 {
		sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", port))
	}
	sshArgs = append(sshArgs, target, "umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys")

	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("cannot read public key: %w", err)
	}

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = strings.NewReader(string(pubKey))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildCopyKeyFallbackCommand(pubKeyPath, user, address string) string {
	return fmt.Sprintf("cat %s | ssh %s@%s 'umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys'",
		pubKeyPath, user, address)
}
```

Note: Need to add `"strings"` to imports in `keys.go`.

**Step 4: Run tests to verify they pass**

Run: `go test ./ssh/ -v`
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add ssh/keys.go ssh/keys_test.go
git commit -m "feat: add SSH key discovery, generation, and copy with fallback"
```

---

## Task 7: tmux — Session Management

**Files:**
- Create: `tmux/sessions.go`
- Create: `tmux/sessions_test.go`

**Step 1: Write failing tests**

```go
package tmux

import (
	"testing"
)

func TestParseSessionList(t *testing.T) {
	// tmux list-sessions -F '#{session_name}|||#{@ccc_project}|||#{@ccc_path}|||#{session_windows}'
	output := `rt1|||rt1|||/Users/mark/Projects/jd/rt1|||3
rt1-feature-x|||rt1|||/Users/mark/Projects/jd/rt1|||1
random-session||||||2`

	sessions := ParseSessionList(output)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	if sessions[0].Name != "rt1" || sessions[0].Project != "rt1" || sessions[0].Windows != 3 {
		t.Errorf("unexpected session 0: %+v", sessions[0])
	}
	if sessions[2].Project != "" {
		t.Errorf("expected empty project for random-session: %+v", sessions[2])
	}
}

func TestFilterSessionsForProject(t *testing.T) {
	sessions := []Session{
		{Name: "rt1", Project: "rt1", Verified: true},
		{Name: "rt1-feature-x", Project: "rt1", Verified: true},
		{Name: "pro-rag", Project: "pro-rag", Verified: true},
		{Name: "rt1-old", Project: "", Verified: false},
	}

	filtered := FilterSessionsForProject(sessions, "rt1")
	if len(filtered) != 3 {
		t.Fatalf("expected 3, got %d", len(filtered))
	}
	// rt1-old should match by prefix but be unverified
	found := false
	for _, s := range filtered {
		if s.Name == "rt1-old" {
			found = true
			if s.Verified {
				t.Error("rt1-old should be unverified")
			}
		}
	}
	if !found {
		t.Error("rt1-old should be included via prefix match")
	}
}

func TestParseEmptyOutput(t *testing.T) {
	sessions := ParseSessionList("")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestBuildCreateCommand(t *testing.T) {
	cmd := BuildCreateCommand("rt1-auth", "/Users/mark/Projects/jd/rt1", "rt1")
	expected := `tmux new-session -d -s rt1-auth -c /Users/mark/Projects/jd/rt1 \; set-option -t rt1-auth @ccc_project rt1 \; set-option -t rt1-auth @ccc_path /Users/mark/Projects/jd/rt1`
	if cmd != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, cmd)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	cmd := BuildAttachCommand("rt1-auth")
	if cmd != "tmux attach -t rt1-auth" {
		t.Errorf("unexpected: %s", cmd)
	}
}

func TestBuildListCommand(t *testing.T) {
	cmd := BuildListCommand()
	if cmd == "" {
		t.Error("expected non-empty command")
	}
}

func TestNextSessionName(t *testing.T) {
	existing := []Session{
		{Name: "rt1"},
		{Name: "rt1-2"},
	}
	name := NextAutoName("rt1", existing)
	if name != "rt1-3" {
		t.Errorf("expected rt1-3, got %s", name)
	}
}

func TestNextSessionNameFirst(t *testing.T) {
	name := NextAutoName("rt1", nil)
	if name != "rt1" {
		t.Errorf("expected rt1, got %s", name)
	}
}

func TestParseClientList(t *testing.T) {
	output := `/dev/ttys001: 220x56 0`
	clients := ParseClientList(output)
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].Width != 220 || clients[0].Height != 56 {
		t.Errorf("unexpected: %+v", clients[0])
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./tmux/ -v`
Expected: FAIL.

**Step 3: Implement tmux sessions**

```go
package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

type Session struct {
	Name     string
	Project  string // from @ccc_project, empty if untagged
	Path     string // from @ccc_path
	Windows  int
	Verified bool // true if matched by metadata, false if by prefix
}

type Client struct {
	TTY    string
	Width  int
	Height int
}

const listFormat = "#{session_name}|||#{@ccc_project}|||#{@ccc_path}|||#{session_windows}"
const clientFormat = "#{client_tty}: #{client_width}x#{client_height} #{client_flags}"

func BuildListCommand() string {
	return fmt.Sprintf("tmux list-sessions -F '%s' 2>/dev/null || true", listFormat)
}

func BuildListClientsCommand(session string) string {
	return fmt.Sprintf("tmux list-clients -t %s -F '%s' 2>/dev/null || true", session, clientFormat)
}

func BuildCreateCommand(name, path, projectKey string) string {
	return fmt.Sprintf(
		`tmux new-session -d -s %s -c %s \; set-option -t %s @ccc_project %s \; set-option -t %s @ccc_path %s`,
		name, path, name, projectKey, name, path,
	)
}

func BuildAttachCommand(name string) string {
	return fmt.Sprintf("tmux attach -t %s", name)
}

func BuildKillCommand(name string) string {
	return fmt.Sprintf("tmux kill-session -t %s", name)
}

func BuildDetachClientsCommand(name string) string {
	return fmt.Sprintf("tmux detach-client -t %s -a", name)
}

func BuildCheckTmuxCommand() string {
	return "command -v tmux"
}

func ParseSessionList(output string) []Session {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	var sessions []Session
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|||", 4)
		if len(parts) < 4 {
			continue
		}
		windows, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
		s := Session{
			Name:     strings.TrimSpace(parts[0]),
			Project:  strings.TrimSpace(parts[1]),
			Path:     strings.TrimSpace(parts[2]),
			Windows:  windows,
			Verified: strings.TrimSpace(parts[1]) != "",
		}
		sessions = append(sessions, s)
	}
	return sessions
}

func FilterSessionsForProject(sessions []Session, projectKey string) []Session {
	var result []Session
	for _, s := range sessions {
		// Match by metadata
		if s.Project == projectKey {
			s.Verified = true
			result = append(result, s)
			continue
		}
		// Fallback: prefix match for untagged sessions
		if s.Project == "" && (s.Name == projectKey || strings.HasPrefix(s.Name, projectKey+"-")) {
			s.Verified = false
			result = append(result, s)
		}
	}
	return result
}

func NextAutoName(projectKey string, existing []Session) string {
	if len(existing) == 0 {
		return projectKey
	}
	// Check if base name is taken
	baseTaken := false
	maxNum := 1
	for _, s := range existing {
		if s.Name == projectKey {
			baseTaken = true
		}
		if strings.HasPrefix(s.Name, projectKey+"-") {
			suffix := strings.TrimPrefix(s.Name, projectKey+"-")
			if n, err := strconv.Atoi(suffix); err == nil && n >= maxNum {
				maxNum = n + 1
			}
		}
	}
	if !baseTaken {
		return projectKey
	}
	if maxNum < 2 {
		maxNum = 2
	}
	return fmt.Sprintf("%s-%d", projectKey, maxNum)
}

func ParseClientList(output string) []Client {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	var clients []Client
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: /dev/ttys001: 220x56 0
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		tty := line[:colonIdx]
		rest := strings.TrimSpace(line[colonIdx+1:])
		parts := strings.Fields(rest)
		if len(parts) < 1 {
			continue
		}
		dims := strings.SplitN(parts[0], "x", 2)
		if len(dims) != 2 {
			continue
		}
		w, _ := strconv.Atoi(dims[0])
		h, _ := strconv.Atoi(dims[1])
		clients = append(clients, Client{TTY: tty, Width: w, Height: h})
	}
	return clients
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tmux/ -v`
Expected: all 9 tests PASS.

**Step 5: Commit**

```bash
git add tmux/
git commit -m "feat: add tmux session management with metadata tagging"
```

---

## Task 8: Scan — Project Discovery

**Files:**
- Create: `scan/projects.go`
- Create: `scan/projects_test.go`

**Step 1: Write failing tests**

```go
package scan

import (
	"strings"
	"testing"
)

func TestBuildMdfindCommand(t *testing.T) {
	cmd := buildMdfindCommand("/Users/mark")
	if !strings.Contains(cmd, "mdfind") {
		t.Error("expected mdfind command")
	}
	if !strings.Contains(cmd, "/Users/mark") {
		t.Error("expected home path")
	}
}

func TestBuildLocateCommand(t *testing.T) {
	cmd := buildLocateCommand("/home/mark")
	if !strings.Contains(cmd, "locate") || !strings.Contains(cmd, "plocate") {
		t.Error("expected locate/plocate command")
	}
}

func TestBuildFdCommand(t *testing.T) {
	cmd := buildFdCommand("/home/mark")
	if !strings.Contains(cmd, "fd") {
		t.Error("expected fd command")
	}
}

func TestBuildFindCommand(t *testing.T) {
	cmd := buildFindCommand("/home/mark")
	if !strings.Contains(cmd, "find") {
		t.Error("expected find command")
	}
	if !strings.Contains(cmd, "maxdepth") {
		t.Error("expected maxdepth")
	}
}

func TestBuildScanChainCommand(t *testing.T) {
	cmd := BuildScanChainCommand("/Users/mark")
	if cmd == "" {
		t.Error("expected non-empty command")
	}
	// Should try mdfind first, fall through on empty
	if !strings.Contains(cmd, "mdfind") {
		t.Error("expected mdfind in chain")
	}
	if !strings.Contains(cmd, "find") {
		t.Error("expected find fallback in chain")
	}
}

func TestParseScanResults(t *testing.T) {
	output := `/Users/mark/Projects/jd/rt1/.git
/Users/mark/Projects/jd/pro-rag/.git
/Users/mark/.cache/something/.git`

	results := ParseScanResults(output)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Should return parent directories, not .git paths
	if results[0].Path != "/Users/mark/Projects/jd/rt1" {
		t.Errorf("expected parent dir, got %s", results[0].Path)
	}
}

func TestDeriveProjectKey(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/Users/mark/Projects/jd/rt1", "rt1"},
		{"/Users/mark/Projects/death_and_taxes", "death-and-taxes"},
		{"/home/user/My Project", "my-project"},
	}
	for _, tt := range tests {
		key := DeriveProjectKey(tt.path)
		if key != tt.expected {
			t.Errorf("DeriveProjectKey(%q) = %q, want %q", tt.path, key, tt.expected)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./scan/ -v`
Expected: FAIL.

**Step 3: Implement scan**

```go
package scan

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ScanResult struct {
	Path string // absolute path to project (parent of .git)
	Name string // directory name
}

// BuildScanChainCommand returns a shell command that tries fast tools first,
// falling through to slower ones if they return empty results.
// Each tool outputs absolute paths to .git entries, one per line.
func BuildScanChainCommand(homeDir string) string {
	mdfind := buildMdfindCommand(homeDir)
	locate := buildLocateCommand(homeDir)
	fd := buildFdCommand(homeDir)
	findCmd := buildFindCommand(homeDir)

	// Chain: try each, use first non-empty result
	return fmt.Sprintf(`result=$(%s); [ -n "$result" ] && echo "$result" && exit 0; `+
		`result=$(%s); [ -n "$result" ] && echo "$result" && exit 0; `+
		`result=$(%s); [ -n "$result" ] && echo "$result" && exit 0; `+
		`%s`,
		mdfind, locate, fd, findCmd)
}

func buildMdfindCommand(homeDir string) string {
	return fmt.Sprintf(
		`command -v mdfind >/dev/null 2>&1 && mdfind "kMDItemFSName == '.git'" -onlyin %s 2>/dev/null`,
		homeDir,
	)
}

func buildLocateCommand(homeDir string) string {
	return fmt.Sprintf(
		`(command -v plocate >/dev/null 2>&1 && plocate -r '%s/.*\.git$' 2>/dev/null) || `+
			`(command -v locate >/dev/null 2>&1 && locate -r '%s/.*\.git$' 2>/dev/null)`,
		homeDir, homeDir,
	)
}

func buildFdCommand(homeDir string) string {
	return fmt.Sprintf(
		`command -v fd >/dev/null 2>&1 && fd -H -t d -t f '^\\.git$' %s --max-depth 4 2>/dev/null`,
		homeDir,
	)
}

func buildFindCommand(homeDir string) string {
	return fmt.Sprintf(
		`find %s -maxdepth 4 \( -name .git \) `+
			`-not -path "*/node_modules/*" `+
			`-not -path "*/Library/*" `+
			`-not -path "*/.cache/*" `+
			`-not -path "*/.Trash/*" `+
			`2>/dev/null`,
		homeDir,
	)
}

// ParseScanResults parses the output of the scan chain.
// Input is lines of absolute paths to .git entries.
// Returns deduplicated project paths (parent of .git).
func ParseScanResults(output string) []ScanResult {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var results []ScanResult
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Get parent directory (strip .git)
		parent := line
		if filepath.Base(line) == ".git" {
			parent = filepath.Dir(line)
		}
		if seen[parent] {
			continue
		}
		seen[parent] = true
		results = append(results, ScanResult{
			Path: parent,
			Name: filepath.Base(parent),
		})
	}
	return results
}

// DeriveProjectKey creates a valid TOML bare key from a path.
// Lowercases, replaces underscores and spaces with hyphens.
func DeriveProjectKey(path string) string {
	name := filepath.Base(path)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var clean strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean.WriteRune(r)
		}
	}
	result := clean.String()
	// Trim leading/trailing hyphens
	result = strings.Trim(result, "-")
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./scan/ -v`
Expected: all 7 tests PASS.

**Step 5: Commit**

```bash
git add scan/
git commit -m "feat: add project discovery with mdfind/locate/fd/find chain"
```

---

## Task 9: Tailscale — Host Discovery

**Files:**
- Create: `tailscale/discovery.go`
- Create: `tailscale/discovery_test.go`

**Step 1: Write failing tests**

```go
package tailscale

import (
	"testing"
)

func TestParseTailscaleStatus(t *testing.T) {
	// Simulated output from `tailscale status`
	output := `100.64.0.1    macbook-m1     mark@        macOS   -
100.64.0.5    server-lab     mark@        linux   -
100.64.0.10   phone          mark@        iOS     -`

	hosts := ParseTailscaleStatus(output)
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
	if hosts[0].Name != "macbook-m1" || hosts[0].IP != "100.64.0.1" {
		t.Errorf("unexpected host 0: %+v", hosts[0])
	}
	if hosts[1].OS != "linux" {
		t.Errorf("unexpected OS: %s", hosts[1].OS)
	}
}

func TestParseTailscaleStatusEmpty(t *testing.T) {
	hosts := ParseTailscaleStatus("")
	if len(hosts) != 0 {
		t.Errorf("expected 0, got %d", len(hosts))
	}
}

func TestParseTailscaleStatusWithSelfLine(t *testing.T) {
	// First line is sometimes the current machine with different format
	output := `100.64.0.1    macbook-m1     mark@        macOS   -
100.64.0.5    server-lab     mark@        linux   active; relay "tok", tx 1234 rx 5678`

	hosts := ParseTailscaleStatus(output)
	if len(hosts) != 2 {
		t.Fatalf("expected 2, got %d", len(hosts))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./tailscale/ -v`
Expected: FAIL.

**Step 3: Implement Tailscale discovery**

```go
package tailscale

import (
	"os/exec"
	"strings"
)

type TailscaleHost struct {
	IP   string
	Name string
	User string
	OS   string
}

// IsAvailable checks if the tailscale CLI is installed.
func IsAvailable() bool {
	_, err := exec.LookPath("tailscale")
	return err == nil
}

// DiscoverHosts runs `tailscale status` and returns parsed hosts.
func DiscoverHosts() ([]TailscaleHost, error) {
	out, err := exec.Command("tailscale", "status", "--peers").Output()
	if err != nil {
		return nil, err
	}
	return ParseTailscaleStatus(string(out)), nil
}

// ParseTailscaleStatus parses the output of `tailscale status`.
func ParseTailscaleStatus(output string) []TailscaleHost {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	var hosts []TailscaleHost
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		hosts = append(hosts, TailscaleHost{
			IP:   fields[0],
			Name: fields[1],
			User: strings.TrimSuffix(fields[2], "@"),
			OS:   fields[3],
		})
	}
	return hosts
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tailscale/ -v`
Expected: all 3 tests PASS.

**Step 5: Commit**

```bash
git add tailscale/
git commit -m "feat: add Tailscale host discovery"
```

---

## Task 10: Main Flow — Local Mode

Wire up local mode first (simplest end-to-end path: no SSH, direct tmux).

**Files:**
- Modify: `main.go`
- Create: `flow/local.go`
- Create: `flow/common.go`

**Step 1: Write flow/common.go — shared flow logic**

```go
package flow

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/markjd/ccc/config"
	"github.com/markjd/ccc/tmux"
	"github.com/markjd/ccc/ui"
)

// Runner abstracts command execution (SSH or local).
type Runner interface {
	Run(cmd string) (string, error)
	RunInteractive(cmd string) error
}

// ProjectFlow handles project selection → session selection → attach/create.
func ProjectFlow(in io.Reader, out io.Writer, runner Runner, projects *config.ProjectsConfig) error {
	keys := projects.SortedProjectKeys()
	if len(keys) == 0 {
		fmt.Fprintf(out, "\n  No projects configured.\n")
		return nil
	}

	items := make([]ui.MenuItem, len(keys))
	for i, k := range keys {
		p := projects.Projects[k]
		items[i] = ui.MenuItem{Key: k, Label: k, Extra: p.Path}
	}

	result, err := ui.ShowMenu(in, out, ui.MenuConfig{
		Title:    "Projects",
		Items:    items,
		ShowBack: true,
		ExtraActions: []ui.ExtraAction{
			{Key: "s", Label: "Scan for projects", Action: "scan"},
		},
	})
	if err != nil {
		return err
	}

	switch result.Action {
	case ui.ActionQuit:
		return nil
	case ui.ActionBack:
		return nil // caller handles re-display
	case ui.ActionExtra:
		if result.ExtraKey == "scan" {
			fmt.Fprintf(out, "\n  Scan not yet implemented in this flow.\n")
			return nil
		}
	case ui.ActionSelect:
		return SessionFlow(in, out, runner, result.Selected.Key, projects.Projects[result.Selected.Key].Path)
	}
	return nil
}

// SessionFlow handles session listing → attach or create.
func SessionFlow(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string) error {
	// List sessions
	listCmd := tmux.BuildListCommand()
	listOutput, err := runner.Run(listCmd)
	if err != nil {
		// Could be "no server" — treat as zero sessions
		listOutput = ""
	}

	allSessions := tmux.ParseSessionList(listOutput)
	sessions := tmux.FilterSessionsForProject(allSessions, projectKey)

	// Auto-skip: zero sessions → create
	if len(sessions) == 0 {
		return createSession(in, out, runner, projectKey, projectPath, sessions)
	}

	// Auto-skip: one session → attach
	if len(sessions) == 1 {
		return attachSession(in, out, runner, sessions[0])
	}

	// Show session menu
	items := make([]ui.MenuItem, len(sessions))
	for i, s := range sessions {
		extra := fmt.Sprintf("(%d windows)", s.Windows)
		if !s.Verified {
			extra += " (unverified)"
		}
		items[i] = ui.MenuItem{Key: s.Name, Label: s.Name, Extra: extra}
	}

	result, err := ui.ShowMenu(in, out, ui.MenuConfig{
		Title:      fmt.Sprintf("Sessions for %s", projectKey),
		Items:      items,
		ShowBack:   true,
		ShowRemove: true,
		ExtraActions: []ui.ExtraAction{
			{Key: "n", Label: "New session", Action: "new"},
		},
	})
	if err != nil {
		return err
	}

	switch result.Action {
	case ui.ActionQuit:
		return nil
	case ui.ActionBack:
		return nil
	case ui.ActionExtra:
		return createSession(in, out, runner, projectKey, projectPath, sessions)
	case ui.ActionRemove:
		return removeSession(in, out, runner, result.Selected, sessions)
	case ui.ActionSelect:
		// Find the session
		for _, s := range sessions {
			if s.Name == result.Selected.Key {
				return attachSession(in, out, runner, s)
			}
		}
	}
	return nil
}

func attachSession(in io.Reader, out io.Writer, runner Runner, session tmux.Session) error {
	if !session.Verified {
		fmt.Fprintf(out, "\n  Session %q matches by name but wasn't created by ccc.\n", session.Name)
		answer, err := ui.Confirm(in, out, "Proceed?")
		if err != nil || !answer {
			return err
		}
	}

	// Check for other clients
	clientOutput, _ := runner.Run(tmux.BuildListClientsCommand(session.Name))
	clients := tmux.ParseClientList(clientOutput)
	if len(clients) > 0 {
		c := clients[0]
		fmt.Fprintf(out, "\n  This session is attached from another client (%dx%d).\n", c.Width, c.Height)

		detachResult, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title: "Options",
			Items: []ui.MenuItem{
				{Key: "attach", Label: fmt.Sprintf("Attach anyway (layout constrained to %dx%d)", c.Width, c.Height)},
				{Key: "detach", Label: "Detach other client and attach (full resolution)"},
			},
		})
		if err != nil {
			return err
		}
		if detachResult.Action == ui.ActionQuit {
			return nil
		}
		if detachResult.Selected.Key == "detach" {
			runner.Run(tmux.BuildDetachClientsCommand(session.Name))
		}
	}

	fmt.Fprintf(out, "\n  Attaching to %s...\n", session.Name)
	return runner.RunInteractive(tmux.BuildAttachCommand(session.Name))
}

func createSession(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string, existing []tmux.Session) error {
	autoName := tmux.NextAutoName(projectKey, existing)
	namePrompt := fmt.Sprintf("Session name (enter for %q)", autoName)
	name, err := ui.Prompt(in, out, namePrompt)
	if err != nil {
		return err
	}
	if name == "" {
		name = autoName
	} else if name != projectKey && !strings.HasPrefix(name, projectKey+"-") {
		name = projectKey + "-" + name
	}

	createCmd := tmux.BuildCreateCommand(name, projectPath, projectKey)
	if _, err := runner.Run(createCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Fprintf(out, "  ✓ Created session %s\n", name)
	return runner.RunInteractive(tmux.BuildAttachCommand(name))
}

func removeSession(in io.Reader, out io.Writer, runner Runner, item ui.MenuItem, sessions []tmux.Session) error {
	// Find session for verification check
	for _, s := range sessions {
		if s.Name == item.Key && !s.Verified {
			fmt.Fprintf(out, "\n  Warning: session %q wasn't created by ccc.\n", s.Name)
		}
	}

	killCmd := tmux.BuildKillCommand(item.Key)
	if _, err := runner.Run(killCmd); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Killed session %s\n", item.Key)
	return nil
}
```

**Step 2: Write flow/local.go**

```go
package flow

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/markjd/ccc/config"
)

// LocalRunner executes commands directly on the local machine.
type LocalRunner struct{}

func (r *LocalRunner) Run(cmd string) (string, error) {
	out, err := exec.Command("bash", "-lc", cmd).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %s\nstderr: %s", cmd, string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *LocalRunner) RunInteractive(cmd string) error {
	proc := exec.Command("bash", "-lc", cmd)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	return proc.Run()
}

// RunLocalMode runs ccc in local mode (no SSH).
func RunLocalMode(in io.Reader, out io.Writer) error {
	// Load projects config from local path
	home, _ := os.UserHomeDir()
	projectsPath := home + "/.ccc/projects.toml"

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "\n  No projects configured. Run scan or create %s\n", projectsPath)
			return nil
		}
		return err
	}

	projects, err := config.ParseProjectsConfig(data)
	if err != nil {
		return fmt.Errorf("config error in %s: %w", projectsPath, err)
	}

	runner := &LocalRunner{}
	return ProjectFlow(in, out, runner, projects)
}

// IsSSHSession checks if we're running inside an SSH session.
func IsSSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""
}
```

**Step 3: Update main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/markjd/ccc/flow"
)

func main() {
	args := os.Args[1:]

	// Check for local mode
	isLocal := len(args) > 0 && args[0] == "local"

	// Auto-detect: if running over SSH, suggest local mode
	if !isLocal && flow.IsSSHSession() {
		fmt.Println("\n  You're already on this machine via SSH.")
		fmt.Println("  Switching to local mode (no SSH hop).")
		isLocal = true
	}

	if isLocal {
		if err := flow.RunLocalMode(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("ccc: remote mode not yet implemented")
}
```

**Step 4: Build and verify**

Run: `go build -o ccc . && ./ccc local`
Expected: either shows projects from `~/.ccc/projects.toml` or says "No projects configured."

**Step 5: Commit**

```bash
git add flow/ main.go
git commit -m "feat: wire up local mode with project and session flows"
```

---

## Task 11: Main Flow — Remote Mode

Wire up the full remote flow: host selection → SSH → project/session flow.

**Files:**
- Create: `flow/remote.go`
- Modify: `main.go`

**Step 1: Implement flow/remote.go**

```go
package flow

import (
	"fmt"
	"io"
	"os"

	"github.com/markjd/ccc/config"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/ui"
)

// SSHRunner executes commands over SSH.
type SSHRunner struct {
	Conn *sshpkg.Connection
}

func (r *SSHRunner) Run(cmd string) (string, error) {
	return r.Conn.RunCommand(cmd)
}

func (r *SSHRunner) RunInteractive(cmd string) error {
	return r.Conn.RunInteractive(cmd)
}

// RunRemoteMode runs ccc in remote mode (SSH to host).
func RunRemoteMode(in io.Reader, out io.Writer, args []string) error {
	cfgPath := config.DefaultClientConfigPath()
	cfg, err := config.LoadClientConfig(cfgPath)
	if err != nil {
		if err == config.ErrNoConfig {
			return runFirstTimeSetup(in, out, cfgPath)
		}
		return fmt.Errorf("config error: %w", err)
	}

	// Shortcut: ccc <project> — skip host if single host
	if len(args) >= 1 && len(cfg.Hosts) == 1 {
		var hostName string
		for name := range cfg.Hosts {
			hostName = name
		}
		return connectToHost(in, out, cfg, hostName, args)
	}

	// Shortcut: ccc <host> <project> [new]
	if len(args) >= 2 {
		if _, ok := cfg.Hosts[args[0]]; ok {
			return connectToHost(in, out, cfg, args[0], args[1:])
		}
	}

	// Interactive host selection
	return hostSelectionLoop(in, out, cfg, cfgPath, args)
}

func hostSelectionLoop(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string, args []string) error {
	for {
		names := cfg.SortedHostNames()
		if len(names) == 0 {
			return runFirstTimeSetup(in, out, cfgPath)
		}

		// Auto-skip single host
		if len(names) == 1 && len(args) == 0 {
			return connectToHost(in, out, cfg, names[0], nil)
		}

		items := make([]ui.MenuItem, len(names))
		for i, name := range names {
			h := cfg.Hosts[name]
			items[i] = ui.MenuItem{
				Key:   name,
				Label: name,
				Extra: fmt.Sprintf("(%s@%s)", h.User, h.Address),
			}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:      "Hosts",
			Items:      items,
			ShowRemove: true,
			ExtraActions: []ui.ExtraAction{
				{Key: "a", Label: "Add host", Action: "add"},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit:
			return nil
		case ui.ActionRemove:
			cfg.RemoveHost(result.Selected.Key)
			config.SaveClientConfig(cfgPath, cfg)
			fmt.Fprintf(out, "  ✓ Removed %s\n", result.Selected.Key)
			continue
		case ui.ActionExtra:
			// Add host flow
			fmt.Fprintf(out, "\n  Add host: not yet fully implemented.\n")
			continue
		case ui.ActionSelect:
			if err := connectToHost(in, out, cfg, result.Selected.Key, nil); err != nil {
				fmt.Fprintf(out, "\n  Error: %v\n", err)
				continue
			}
			return nil
		}
	}
}

func connectToHost(in io.Reader, out io.Writer, cfg *config.ClientConfig, hostName string, args []string) error {
	host, ok := cfg.Hosts[hostName]
	if !ok {
		return fmt.Errorf("unknown host: %s", hostName)
	}

	conn := &sshpkg.Connection{
		User:         host.User,
		Address:      host.Address,
		Port:         host.Port,
		IdentityFile: host.IdentityFile,
		ProxyJump:    host.ProxyJump,
		SSHOptions:   host.SSHOptions,
	}

	fmt.Fprintf(out, "\n  Connecting to %s...\n", hostName)

	// Read projects config from host
	projectsData, err := conn.RunCommand("cat ~/.ccc/projects.toml")
	if err != nil {
		// Check if it's a connection issue or missing file
		if testErr := conn.TestConnection(); testErr != nil {
			return fmt.Errorf("cannot reach %s: %w", hostName, testErr)
		}
		// File missing → trigger scan
		fmt.Fprintf(out, "  No projects configured on %s.\n", hostName)
		return runRemoteScan(in, out, conn, hostName)
	}

	projects, err := config.ParseProjectsConfig([]byte(projectsData))
	if err != nil {
		return fmt.Errorf("projects config error on %s: %w", hostName, err)
	}

	runner := &SSHRunner{Conn: conn}

	// Shortcut: project specified as arg
	if len(args) >= 1 {
		projectKey := args[0]
		if p, ok := projects.Projects[projectKey]; ok {
			if len(args) >= 2 && args[1] == "new" {
				return createSession(in, out, runner, projectKey, p.Path, nil)
			}
			return SessionFlow(in, out, runner, projectKey, p.Path)
		}
		fmt.Fprintf(out, "  Unknown project: %s\n", projectKey)
	}

	return ProjectFlow(in, out, runner, projects)
}

func runFirstTimeSetup(in io.Reader, out io.Writer, cfgPath string) error {
	fmt.Fprintf(out, "\n  No config found. Let's set up your first host.\n")
	// TODO: Tailscale discovery, manual entry, save config
	fmt.Fprintf(out, "  First-time setup not yet fully implemented.\n")
	return nil
}

func runRemoteScan(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) error {
	// TODO: run scan chain, present results, save projects.toml
	fmt.Fprintf(out, "  Remote scan not yet fully implemented.\n")
	return nil
}
```

**Step 2: Update main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/markjd/ccc/flow"
)

func main() {
	args := os.Args[1:]

	// Check for local mode
	isLocal := len(args) > 0 && args[0] == "local"
	if isLocal {
		args = args[1:]
	}

	// Auto-detect: if running over SSH, use local mode
	if !isLocal && flow.IsSSHSession() {
		fmt.Println("\n  You're already on this machine via SSH.")
		fmt.Println("  Switching to local mode (no SSH hop).")
		isLocal = true
	}

	if isLocal {
		if err := flow.RunLocalMode(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := flow.RunRemoteMode(os.Stdin, os.Stdout, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Build and verify**

Run: `go build -o ccc .`
Expected: builds without errors.

**Step 4: Commit**

```bash
git add flow/remote.go main.go
git commit -m "feat: wire up remote mode with host selection and SSH"
```

---

## Task 12: First-Time Setup — Host Addition with Tailscale

Complete the first-time setup and "Add host" flows.

**Files:**
- Create: `flow/setup.go`
- Modify: `flow/remote.go` — replace stub functions

**Step 1: Implement flow/setup.go**

```go
package flow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/markjd/ccc/config"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/tailscale"
	"github.com/markjd/ccc/ui"
)

// SetupFirstHost walks through adding the first host.
func SetupFirstHost(in io.Reader, out io.Writer, cfgPath string) (*config.ClientConfig, error) {
	fmt.Fprintf(out, "\n  No config found. Let's set up your first host.\n")

	name, host, err := addHostInteractive(in, out)
	if err != nil {
		return nil, err
	}

	cfg := &config.ClientConfig{Hosts: map[string]config.Host{}}
	cfg.AddHost(name, host)

	if err := config.SaveClientConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Host saved.\n")
	return cfg, nil
}

// AddHostFlow adds a new host to an existing config.
func AddHostFlow(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string) error {
	name, host, err := addHostInteractive(in, out)
	if err != nil {
		return err
	}
	cfg.AddHost(name, host)
	if err := config.SaveClientConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Host saved.\n")
	return nil
}

func addHostInteractive(in io.Reader, out io.Writer) (string, config.Host, error) {
	var name string
	var host config.Host

	// Try Tailscale discovery
	if tailscale.IsAvailable() {
		fmt.Fprintf(out, "\n  Looking for Tailscale... found.\n")
		hosts, err := tailscale.DiscoverHosts()
		if err == nil && len(hosts) > 0 {
			items := make([]ui.MenuItem, len(hosts))
			for i, h := range hosts {
				items[i] = ui.MenuItem{
					Key:   h.Name,
					Label: h.Name,
					Extra: fmt.Sprintf("%s (%s)", h.IP, h.OS),
				}
			}
			items = append(items, ui.MenuItem{Key: "_manual", Label: "Enter manually"})

			result, err := ui.ShowMenu(in, out, ui.MenuConfig{
				Title: "Tailscale hosts",
				Items: items,
			})
			if err != nil {
				return "", host, err
			}
			if result.Action == ui.ActionQuit {
				return "", host, fmt.Errorf("cancelled")
			}
			if result.Selected.Key != "_manual" {
				// Found via Tailscale
				for _, h := range hosts {
					if h.Name == result.Selected.Key {
						name = h.Name
						host.Address = h.IP
						break
					}
				}
				user, err := ui.Prompt(in, out, fmt.Sprintf("SSH user for %s", name))
				if err != nil {
					return "", host, err
				}
				host.User = user
				return name, host, testAndSetupKeys(in, out, name, &host)
			}
		}
	} else {
		fmt.Fprintf(out, "\n  Looking for Tailscale... not found.\n")
	}

	// Manual entry
	fmt.Fprintf(out, "\n  Enter host details manually:\n")
	var err error
	name, err = ui.Prompt(in, out, "Name")
	if err != nil {
		return "", host, err
	}
	host.User, err = ui.Prompt(in, out, "User")
	if err != nil {
		return "", host, err
	}
	host.Address, err = ui.Prompt(in, out, "Address")
	if err != nil {
		return "", host, err
	}

	return name, host, testAndSetupKeys(in, out, name, &host)
}

func testAndSetupKeys(in io.Reader, out io.Writer, hostName string, host *config.Host) error {
	conn := &sshpkg.Connection{
		User:    host.User,
		Address: host.Address,
		Port:    host.Port,
	}

	fmt.Fprintf(out, "  Testing connection...\n")
	if err := conn.TestConnection(); err != nil {
		fmt.Fprintf(out, "  Authentication failed for %s@%s.\n", host.User, host.Address)

		result, menuErr := ui.ShowMenu(in, out, ui.MenuConfig{
			Title: "Options",
			Items: []ui.MenuItem{
				{Key: "keys", Label: "Set up SSH keys now"},
				{Key: "user", Label: "Try a different user"},
				{Key: "cancel", Label: "Cancel"},
			},
		})
		if menuErr != nil {
			return menuErr
		}

		switch result.Selected.Key {
		case "keys":
			return setupKeysFlow(in, out, host)
		case "user":
			newUser, err := ui.Prompt(in, out, "User")
			if err != nil {
				return err
			}
			host.User = newUser
			return testAndSetupKeys(in, out, hostName, host)
		default:
			return fmt.Errorf("cancelled")
		}
	}

	fmt.Fprintf(out, "  ✓ Connected.\n")
	return nil
}

func setupKeysFlow(in io.Reader, out io.Writer, host *config.Host) error {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

	pubKey, err := sshpkg.FindExistingPublicKey(sshDir)
	if err != nil {
		return err
	}

	if pubKey == "" {
		fmt.Fprintf(out, "  No SSH key found. Generating...\n")
		pubKey, err = sshpkg.GenerateKey(sshDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  ✓ Created key.\n")
	} else {
		fmt.Fprintf(out, "  Found %s\n", pubKey)
	}

	fmt.Fprintf(out, "  Copying public key to host...\n")
	if err := sshpkg.CopyKeyToHost(pubKey, host.User, host.Address, host.Port); err != nil {
		return fmt.Errorf("key copy failed: %w", err)
	}

	fmt.Fprintf(out, "  ✓ Key installed. Testing connection...\n")
	conn := &sshpkg.Connection{User: host.User, Address: host.Address, Port: host.Port}
	if err := conn.TestConnection(); err != nil {
		return fmt.Errorf("still cannot connect after key setup: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Connected.\n")
	return nil
}
```

**Step 2: Update flow/remote.go — replace stubs**

Replace the `runFirstTimeSetup` function body:

```go
func runFirstTimeSetup(in io.Reader, out io.Writer, cfgPath string) error {
	cfg, err := SetupFirstHost(in, out, cfgPath)
	if err != nil {
		return err
	}
	return hostSelectionLoop(in, out, cfg, cfgPath, nil)
}
```

Replace the "add" extra action in `hostSelectionLoop`:

```go
case ui.ActionExtra:
	if err := AddHostFlow(in, out, cfg, cfgPath); err != nil {
		fmt.Fprintf(out, "  %v\n", err)
	}
	// Reload config
	cfg, _ = config.LoadClientConfig(cfgPath)
	continue
```

**Step 3: Build and verify**

Run: `go build -o ccc .`
Expected: builds without errors.

**Step 4: Commit**

```bash
git add flow/setup.go flow/remote.go
git commit -m "feat: add first-time setup with Tailscale discovery and SSH key flow"
```

---

## Task 13: Remote Scan — Project Discovery over SSH

Complete the remote scan flow.

**Files:**
- Modify: `flow/remote.go` — replace `runRemoteScan` stub
- Create: `flow/scan.go`

**Step 1: Implement flow/scan.go**

```go
package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/markjd/ccc/config"
	"github.com/markjd/ccc/scan"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/ui"
)

// RunScanFlow discovers projects on a host and saves projects.toml.
func RunScanFlow(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) (*config.ProjectsConfig, error) {
	// Get home directory
	homeDir, err := conn.RunCommand("echo $HOME")
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Fprintf(out, "\n  Scanning for git repositories in %s...\n", homeDir)
	scanCmd := scan.BuildScanChainCommand(homeDir)
	scanOutput, err := conn.RunCommand(scanCmd)
	if err != nil || strings.TrimSpace(scanOutput) == "" {
		return handleNoResults(in, out, conn, hostName)
	}

	results := scan.ParseScanResults(scanOutput)
	if len(results) == 0 {
		return handleNoResults(in, out, conn, hostName)
	}

	return selectProjects(in, out, conn, results)
}

func handleNoResults(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) (*config.ProjectsConfig, error) {
	fmt.Fprintf(out, "\n  No git repositories found.\n")

	result, err := ui.ShowMenu(in, out, ui.MenuConfig{
		Title: "Options",
		Items: []ui.MenuItem{
			{Key: "path", Label: "Enter a path to scan"},
			{Key: "shell", Label: fmt.Sprintf("Open a shell on %s", hostName)},
		},
	})
	if err != nil || result.Action == ui.ActionQuit {
		return nil, err
	}

	if result.Selected.Key == "shell" {
		fmt.Fprintf(out, "\n  Opening shell. Type 'exit' when done.\n")
		conn.RunInteractive("bash -l")

		// Rescan after shell exit
		fmt.Fprintf(out, "\n  Rescanning...\n")
		homeDir, _ := conn.RunCommand("echo $HOME")
		scanCmd := scan.BuildScanChainCommand(homeDir)
		scanOutput, _ := conn.RunCommand(scanCmd)
		results := scan.ParseScanResults(scanOutput)
		if len(results) == 0 {
			fmt.Fprintf(out, "  Still no projects found.\n")
			return nil, nil
		}
		return selectProjects(in, out, conn, results)
	}

	// Manual path
	path, err := ui.Prompt(in, out, "Path to scan")
	if err != nil {
		return nil, err
	}
	scanCmd := scan.BuildScanChainCommand(path)
	scanOutput, _ := conn.RunCommand(scanCmd)
	results := scan.ParseScanResults(scanOutput)
	if len(results) == 0 {
		fmt.Fprintf(out, "  No projects found at %s.\n", path)
		return nil, nil
	}
	return selectProjects(in, out, conn, results)
}

func selectProjects(in io.Reader, out io.Writer, conn *sshpkg.Connection, results []scan.ScanResult) (*config.ProjectsConfig, error) {
	fmt.Fprintf(out, "\n  Found %d projects:\n", len(results))
	for i, r := range results {
		fmt.Fprintf(out, "  [%d] %-20s %s\n", i+1, r.Name, r.Path)
	}

	answer, err := ui.Prompt(in, out, "Select projects to add (comma-separated, or 'a' for all)")
	if err != nil {
		return nil, err
	}

	projects := &config.ProjectsConfig{Projects: map[string]config.Project{}}

	if answer == "a" || answer == "A" {
		for _, r := range results {
			key := scan.DeriveProjectKey(r.Path)
			projects.Projects[key] = config.Project{Path: r.Path}
		}
	} else {
		for _, part := range strings.Split(answer, ",") {
			part = strings.TrimSpace(part)
			idx := 0
			fmt.Sscanf(part, "%d", &idx)
			if idx >= 1 && idx <= len(results) {
				r := results[idx-1]
				key := scan.DeriveProjectKey(r.Path)
				projects.Projects[key] = config.Project{Path: r.Path}
			}
		}
	}

	if len(projects.Projects) == 0 {
		fmt.Fprintf(out, "  No projects selected.\n")
		return nil, nil
	}

	// Save to host
	data, err := config.SerializeProjectsConfig(projects)
	if err != nil {
		return nil, err
	}
	writeCmd := fmt.Sprintf("mkdir -p ~/.ccc && cat > ~/.ccc/projects.toml << 'CCCEOF'\n%s\nCCCEOF", string(data))
	if _, err := conn.RunCommand(writeCmd); err != nil {
		return nil, fmt.Errorf("failed to write projects.toml: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Saved %d projects to ~/.ccc/projects.toml\n", len(projects.Projects))

	return projects, nil
}
```

**Step 2: Update flow/remote.go — replace runRemoteScan**

```go
func runRemoteScan(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) error {
	projects, err := RunScanFlow(in, out, conn, hostName)
	if err != nil || projects == nil {
		return err
	}
	runner := &SSHRunner{Conn: conn}
	return ProjectFlow(in, out, runner, projects)
}
```

**Step 3: Build and verify**

Run: `go build -o ccc .`
Expected: builds without errors.

**Step 4: Commit**

```bash
git add flow/scan.go flow/remote.go
git commit -m "feat: add remote project scanning with fallback chain"
```

---

## Task 14: Error Handling — tmux Not Installed

**Files:**
- Create: `flow/errors.go`
- Modify: `flow/common.go` — add tmux check before session operations

**Step 1: Implement flow/errors.go**

```go
package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/markjd/ccc/tmux"
)

// CheckTmux verifies tmux is available. If not, shows install instructions
// and offers a shell to install it.
func CheckTmux(in io.Reader, out io.Writer, runner Runner) error {
	result, err := runner.Run(tmux.BuildCheckTmuxCommand())
	if err == nil && strings.TrimSpace(result) != "" {
		return nil // tmux found
	}

	// Detect OS for install instructions
	osInfo, _ := runner.Run("uname -s")
	osInfo = strings.TrimSpace(strings.ToLower(osInfo))

	fmt.Fprintf(out, "\n  tmux not found.\n\n")
	fmt.Fprintf(out, "  Install tmux:\n")

	switch {
	case strings.Contains(osInfo, "darwin"):
		fmt.Fprintf(out, "    macOS:   brew install tmux\n")
	case strings.Contains(osInfo, "linux"):
		fmt.Fprintf(out, "    Ubuntu:  sudo apt install tmux\n")
		fmt.Fprintf(out, "    Fedora:  sudo dnf install tmux\n")
		fmt.Fprintf(out, "    Arch:    sudo pacman -S tmux\n")
	default:
		fmt.Fprintf(out, "    macOS:   brew install tmux\n")
		fmt.Fprintf(out, "    Ubuntu:  sudo apt install tmux\n")
		fmt.Fprintf(out, "    Fedora:  sudo dnf install tmux\n")
		fmt.Fprintf(out, "    Arch:    sudo pacman -S tmux\n")
		fmt.Fprintf(out, "    Windows: Not supported (use WSL)\n")
	}

	fmt.Fprintf(out, "\n  Opening shell so you can install it...\n")
	if err := runner.RunInteractive("bash -l"); err != nil {
		return fmt.Errorf("shell failed: %w", err)
	}

	// Recheck
	fmt.Fprintf(out, "\n  Rechecking... ")
	result, err = runner.Run(tmux.BuildCheckTmuxCommand())
	if err != nil || strings.TrimSpace(result) == "" {
		fmt.Fprintf(out, "tmux still not found.\n")
		return fmt.Errorf("tmux not installed")
	}
	fmt.Fprintf(out, "✓ tmux found.\n")
	return nil
}
```

**Step 2: Add tmux check in flow/common.go SessionFlow**

At the top of `SessionFlow`, before listing sessions:

```go
// Check tmux is available
if err := CheckTmux(in, out, runner); err != nil {
	return err
}
```

**Step 3: Build and verify**

Run: `go build -o ccc .`
Expected: builds without errors.

**Step 4: Commit**

```bash
git add flow/errors.go flow/common.go
git commit -m "feat: add tmux installation check with OS-specific instructions"
```

---

## Task 15: End-to-End Test + Polish

Manual end-to-end test and final polish.

**Files:**
- Modify: various files for any issues found during testing

**Step 1: Test local mode end-to-end**

Create a test projects.toml:
```bash
mkdir -p ~/.ccc
cat > ~/.ccc/projects.toml << 'EOF'
[projects.ccc]
path = "/Users/mark/Projects/jd/ccc"
EOF
```

Run: `go build -o ccc . && ./ccc local`
Expected: shows project list, can select `ccc`, can create and attach a tmux session.

**Step 2: Test error cases**

- Run with no config: `rm ~/.ccc/config.toml 2>/dev/null; ./ccc`
- Run in SSH session: `SSH_CONNECTION="1 2 3 4" ./ccc`

**Step 3: Verify builds cleanly**

Run: `go vet ./... && go build -o ccc .`
Expected: no warnings, clean build.

**Step 4: Run all tests**

Run: `go test ./... -v`
Expected: all tests pass.

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "chore: end-to-end testing polish and fixes"
```

---

## Task 16: Cross-Compilation

Build for all target platforms.

**Step 1: Build for macOS ARM (primary)**

Run: `GOOS=darwin GOARCH=arm64 go build -o ccc-darwin-arm64 .`

**Step 2: Build for macOS Intel**

Run: `GOOS=darwin GOARCH=amd64 go build -o ccc-darwin-amd64 .`

**Step 3: Build for Linux ARM (for potential Linux hosts)**

Run: `GOOS=linux GOARCH=arm64 go build -o ccc-linux-arm64 .`

**Step 4: Build for Linux Intel**

Run: `GOOS=linux GOARCH=amd64 go build -o ccc-linux-amd64 .`

**Step 5: Verify the local binary works**

Run: `./ccc-darwin-arm64 local`
Expected: works identically to `./ccc`.

**Step 6: Add .gitignore and commit**

```gitignore
ccc
ccc-*
```

```bash
git add .gitignore
git commit -m "chore: add cross-compilation targets and gitignore"
```

---

## Summary

| Task | Component | What it does |
|------|-----------|-------------|
| 1 | Skeleton | Go module + main.go |
| 2 | UI | Menu system with numbered selection |
| 3 | Config | Client config (hosts) read/write |
| 4 | Config | Projects config parse/serialize |
| 5 | SSH | Command execution with BatchMode |
| 6 | SSH | Key discovery, generation, copy |
| 7 | tmux | Session list/create/attach/kill with metadata |
| 8 | Scan | Project discovery chain (mdfind/locate/fd/find) |
| 9 | Tailscale | Host discovery |
| 10 | Flow | Local mode (full end-to-end) |
| 11 | Flow | Remote mode (host → project → session) |
| 12 | Flow | First-time setup + add host |
| 13 | Flow | Remote project scanning |
| 14 | Flow | tmux-not-installed error handling |
| 15 | Test | End-to-end testing + polish |
| 16 | Build | Cross-compilation |

Tasks 1-9 are independent building blocks (libraries). Tasks 10-14 wire them together into the full flow. Tasks 15-16 are final verification.
