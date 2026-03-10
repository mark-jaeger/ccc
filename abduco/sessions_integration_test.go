//go:build integration

package abduco_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mark-jaeger/ccc/abduco"
)

// skipIfNoAbduco skips the test if abduco is not installed
func skipIfNoAbduco(t *testing.T) {
	if _, err := exec.LookPath("abduco"); err != nil {
		t.Skip("abduco not installed, skipping integration test")
	}
}

// runCommand executes a shell command and returns stdout
func runCommand(cmd string) (string, error) {
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	return string(out), err
}

// cleanupSession kills a session by PID, ignoring errors
func cleanupSession(pid int) {
	if pid > 0 {
		cmd := abduco.BuildKillCommand(pid)
		exec.Command("bash", "-c", cmd).Run()
	}
}

// findSessionPID finds the PID of a session by name
func findSessionPID(name string) int {
	listCmd := abduco.BuildListCommand()
	output, err := runCommand(listCmd)
	if err != nil {
		return 0
	}

	sessions := abduco.ParseSessionList(output)
	for _, s := range sessions {
		if s.Name == name {
			return s.PID
		}
	}
	return 0
}

func TestIntegrationCreateAndList(t *testing.T) {
	skipIfNoAbduco(t)

	sessionName := fmt.Sprintf("ccc.test.%d", time.Now().UnixNano())
	var sessionPID int

	defer func() {
		cleanupSession(sessionPID)
	}()

	// Create session using the command builder
	createCmd := abduco.BuildCreateCommand(sessionName, "/tmp", "")
	_, err := runCommand(createCmd)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Brief pause to allow session to start
	time.Sleep(100 * time.Millisecond)

	// List sessions and verify ours appears
	listCmd := abduco.BuildListCommand()
	output, err := runCommand(listCmd)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	sessions := abduco.ParseSessionList(output)
	var found bool
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			sessionPID = s.PID
			if s.External {
				t.Error("Session should not be External (has ccc. prefix)")
			}
			if s.Project != "test" {
				t.Errorf("Session Project = %q, want %q", s.Project, "test")
			}
			break
		}
	}

	if !found {
		t.Errorf("Created session %q not found in list output:\n%s", sessionName, output)
	}
}

func TestIntegrationKillSession(t *testing.T) {
	skipIfNoAbduco(t)

	sessionName := fmt.Sprintf("ccc.testkill.%d", time.Now().UnixNano())

	// Create session
	createCmd := abduco.BuildCreateCommand(sessionName, "/tmp", "")
	_, err := runCommand(createCmd)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Brief pause to allow session to start
	time.Sleep(100 * time.Millisecond)

	// Find the session PID
	pid := findSessionPID(sessionName)
	if pid == 0 {
		t.Fatal("Failed to find created session PID")
	}

	// Kill the session
	killCmd := abduco.BuildKillCommand(pid)
	_, err = runCommand(killCmd)
	if err != nil {
		t.Fatalf("Failed to kill session: %v", err)
	}

	// Brief pause to allow session to terminate
	time.Sleep(100 * time.Millisecond)

	// Verify session is gone (or dead)
	listCmd := abduco.BuildListCommand()
	output, err := runCommand(listCmd)
	if err != nil {
		t.Fatalf("Failed to list sessions after kill: %v", err)
	}

	sessions := abduco.ParseSessionList(output)
	for _, s := range sessions {
		if s.Name == sessionName && !s.Dead {
			t.Errorf("Session %q still alive after kill", sessionName)
		}
	}
}

func TestIntegrationCheckCommand(t *testing.T) {
	skipIfNoAbduco(t)

	// The check command should succeed if abduco is installed
	checkCmd := abduco.BuildCheckCommand()
	output, err := runCommand(checkCmd)
	if err != nil {
		t.Errorf("Check command failed: %v", err)
	}

	if !strings.Contains(output, "abduco") {
		t.Errorf("Check command output should contain 'abduco', got: %s", output)
	}
}

func TestIntegrationFilterForProject(t *testing.T) {
	skipIfNoAbduco(t)

	// Create two sessions with the same project
	projectKey := fmt.Sprintf("proj%d", time.Now().UnixNano()%10000)
	session1 := fmt.Sprintf("ccc.%s.main", projectKey)
	session2 := fmt.Sprintf("ccc.%s.2", projectKey)
	externalSession := fmt.Sprintf("external-%d", time.Now().UnixNano())

	var pids []int
	defer func() {
		for _, pid := range pids {
			cleanupSession(pid)
		}
	}()

	// Create sessions
	for _, name := range []string{session1, session2, externalSession} {
		cmd := abduco.BuildCreateCommand(name, "/tmp", "")
		_, err := runCommand(cmd)
		if err != nil {
			t.Fatalf("Failed to create session %q: %v", name, err)
		}
	}

	// Brief pause to allow sessions to start
	time.Sleep(200 * time.Millisecond)

	// List and collect PIDs for cleanup
	listCmd := abduco.BuildListCommand()
	output, _ := runCommand(listCmd)
	sessions := abduco.ParseSessionList(output)

	for _, s := range sessions {
		if s.Name == session1 || s.Name == session2 || s.Name == externalSession {
			pids = append(pids, s.PID)
		}
	}

	// Filter for our project
	filtered := abduco.FilterSessionsForProject(sessions, projectKey)

	// Should include both project sessions and external session
	foundMain := false
	found2 := false
	foundExternal := false

	for _, s := range filtered {
		if s.Name == session1 {
			foundMain = true
		}
		if s.Name == session2 {
			found2 = true
		}
		if s.Name == externalSession {
			foundExternal = true
		}
	}

	if !foundMain {
		t.Errorf("FilterSessionsForProject missing %s", session1)
	}
	if !found2 {
		t.Errorf("FilterSessionsForProject missing %s", session2)
	}
	if !foundExternal {
		t.Errorf("FilterSessionsForProject missing external session %s", externalSession)
	}
}
