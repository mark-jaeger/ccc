//go:build integration

package flow_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/flow"
	"github.com/mark-jaeger/ccc/internal/testutil"
	"github.com/mark-jaeger/ccc/zmx"
)

// slowReader wraps an io.Reader and returns at most one byte per Read call.
// This prevents bufio.Scanner from buffering the entire input, allowing
// multiple sequential scanners to read from the same underlying reader.
type slowReader struct {
	r io.Reader
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return s.r.Read(p[:1])
}

func newSlowReader(s string) *slowReader {
	return &slowReader{strings.NewReader(s)}
}

func TestProjectFlow_CreateNewSession(t *testing.T) {
	t.Skip("TODO: update testutil for zmx")

	t.Parallel()
	tt := testutil.NewTestTmux(t)

	projects := &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "myapp", Path: "/tmp"},
		},
	}

	// Input: select project 1 (no name prompt - auto-naming)
	in := newSlowReader("1\n")
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	// Verify session was actually created
	if !tt.SessionExists(t, "ccc.myapp.main") {
		t.Error("expected session 'ccc.myapp.main' to exist")
	}
}

func TestProjectFlow_AttachExistingSession(t *testing.T) {
	t.Skip("TODO: update testutil for zmx")

	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Pre-create a session using zmx command
	createCmd := zmx.BuildCreateCommand("ccc.myapp.main", "/tmp")
	if _, err := tt.Run(createCmd); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	projects := &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "myapp", Path: "/tmp"},
		},
	}

	// Input: select project 1, then select session 1
	in := newSlowReader("1\n1\n")
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	if !strings.Contains(out.String(), "Attaching") {
		t.Errorf("expected attach message, got: %s", out.String())
	}
}

func TestProjectFlow_MultipleSessionsFiltered(t *testing.T) {
	t.Skip("TODO: update testutil for zmx")

	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create sessions for two different projects
	cmd1 := zmx.BuildCreateCommand("ccc.myapp.main", "/tmp")
	if _, err := tt.Run(cmd1); err != nil {
		t.Fatalf("create myapp failed: %v", err)
	}
	cmd2 := zmx.BuildCreateCommand("ccc.other.main", "/tmp")
	if _, err := tt.Run(cmd2); err != nil {
		t.Fatalf("create other failed: %v", err)
	}

	projects := &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "myapp", Path: "/tmp"},
		},
	}

	// Input: select project 1, then select session 1 from session menu
	in := newSlowReader("1\n1\n")
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected myapp in output, got: %s", output)
	}
	if strings.Contains(output, "Sessions for other") {
		t.Errorf("should not show sessions for 'other' project, got: %s", output)
	}
}

func TestSessionFlow_KillSession(t *testing.T) {
	t.Skip("TODO: update testutil for zmx")

	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create a session
	cmd := zmx.BuildCreateCommand("ccc.myapp.main", "/tmp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	// Input: select 'x' for kill, select session 1, confirm 'y', then quit
	in := newSlowReader("x\n1\ny\nq\n")
	out := &bytes.Buffer{}

	err := flow.SessionFlow(in, out, tt, "myapp", "/tmp")
	if err != nil {
		t.Fatalf("SessionFlow error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Killed session") {
		t.Errorf("expected kill success message, got: %s", output)
	}
}
