package ui

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildActionBarBasic(t *testing.T) {
	bar := buildActionBar(MenuConfig{
		Title: "Test",
		Items: []MenuItem{{Key: "a", Label: "Item A"}},
	})
	if !strings.Contains(bar, "[enter] Select") {
		t.Errorf("expected [enter] Select, got: %s", bar)
	}
	if !strings.Contains(bar, "[q] Quit") {
		t.Errorf("expected [q] Quit, got: %s", bar)
	}
	if strings.Contains(bar, "[b]") {
		t.Error("should not contain [b] Back when ShowBack is false")
	}
	if strings.Contains(bar, "[r]") {
		t.Error("should not contain [r] Remove when ShowRemove is false")
	}
}

func TestBuildActionBarFull(t *testing.T) {
	bar := buildActionBar(MenuConfig{
		Title:      "Test",
		Items:      []MenuItem{{Key: "a", Label: "Item A"}},
		ShowBack:   true,
		ShowRemove: true,
		ExtraActions: []ExtraAction{
			{Key: "n", Label: "New", ID: "new"},
			{Key: "d", Label: "Detach", ID: "detach", ItemAction: true},
		},
	})
	if !strings.Contains(bar, "[enter] Select") {
		t.Errorf("expected [enter] Select, got: %s", bar)
	}
	if !strings.Contains(bar, "[n] New") {
		t.Errorf("expected [n] New, got: %s", bar)
	}
	if !strings.Contains(bar, "[d] Detach") {
		t.Errorf("expected [d] Detach, got: %s", bar)
	}
	if !strings.Contains(bar, "[r] Remove") {
		t.Errorf("expected [r] Remove, got: %s", bar)
	}
	if !strings.Contains(bar, "[b] Back") {
		t.Errorf("expected [b] Back, got: %s", bar)
	}
	if !strings.Contains(bar, "[q] Quit") {
		t.Errorf("expected [q] Quit, got: %s", bar)
	}
}

func TestRenderMenuOutput(t *testing.T) {
	// Write to a temp file so we can read back the ANSI output
	f, err := os.CreateTemp("", "render-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	cfg := MenuConfig{
		Title: "Sessions",
		Items: []MenuItem{
			{Key: "s1", Label: "myapp", Extra: "(2 windows)"},
			{Key: "s2", Label: "myapp-2", Extra: "(1 window)"},
		},
		ShowBack:   true,
		ShowRemove: true,
	}
	bar := buildActionBar(cfg)

	lines := renderMenu(f, cfg, 0, bar)

	// Newlines emitted: blank + title + 2 items + blank = 5 (action bar has no trailing \n)
	if lines != 5 {
		t.Errorf("expected 5 lines, got %d", lines)
	}

	// Read back the output
	f.Seek(0, 0)
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Sessions") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "myapp") {
		t.Error("expected item 'myapp' in output")
	}
	if !strings.Contains(output, "myapp-2") {
		t.Error("expected item 'myapp-2' in output")
	}
	// First item should be highlighted (reverse video)
	if !strings.Contains(output, "\x1b[7m> myapp") {
		t.Error("expected first item highlighted with reverse video")
	}
}

func TestRenderMenuCursorPosition(t *testing.T) {
	f, err := os.CreateTemp("", "render-cursor-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	cfg := MenuConfig{
		Title: "Test",
		Items: []MenuItem{
			{Key: "a", Label: "Alpha"},
			{Key: "b", Label: "Beta"},
			{Key: "c", Label: "Gamma"},
		},
	}
	bar := buildActionBar(cfg)

	renderMenu(f, cfg, 1, bar) // cursor on Beta

	f.Seek(0, 0)
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	output := string(buf[:n])

	// Beta should be highlighted
	if !strings.Contains(output, "\x1b[7m> Beta") {
		t.Error("expected Beta highlighted")
	}
	// Alpha and Gamma should not be highlighted
	if strings.Contains(output, "\x1b[7m> Alpha") {
		t.Error("Alpha should not be highlighted")
	}
	if strings.Contains(output, "\x1b[7m> Gamma") {
		t.Error("Gamma should not be highlighted")
	}
}

func TestMenuItemActionLineFallback(t *testing.T) {
	// Test that ItemAction extras work in line-based mode
	in := strings.NewReader("d\n1\n")
	out := &strings.Builder{}

	result, err := showLineMenu(in, out, MenuConfig{
		Title: "Sessions",
		Items: []MenuItem{
			{Key: "s1", Label: "session1"},
			{Key: "s2", Label: "session2"},
		},
		ExtraActions: []ExtraAction{
			{Key: "d", Label: "Detach", ID: "detach", ItemAction: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionExtra || result.ExtraKey != "detach" {
		t.Errorf("expected extra detach, got %+v", result)
	}
	if result.Selected.Key != "s1" {
		t.Errorf("expected selected s1, got %+v", result.Selected)
	}
}

func TestReadKeyArrowUp(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{0x1b, '[', 'A'})
	w.Close()
	evt, _ := readKey(r)
	if evt != keyUp {
		t.Errorf("expected keyUp, got %d", evt)
	}
}

func TestReadKeyArrowDown(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{0x1b, '[', 'B'})
	w.Close()
	evt, _ := readKey(r)
	if evt != keyDown {
		t.Errorf("expected keyDown, got %d", evt)
	}
}

func TestReadKeyEnter(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{13})
	w.Close()
	evt, _ := readKey(r)
	if evt != keyEnter {
		t.Errorf("expected keyEnter, got %d", evt)
	}
}

func TestReadKeyCtrlC(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{3})
	w.Close()
	evt, _ := readKey(r)
	if evt != keyCtrlC {
		t.Errorf("expected keyCtrlC, got %d", evt)
	}
}

func TestReadKeyEsc(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{27})
	w.Close()
	evt, _ := readKey(r)
	if evt != keyEsc {
		t.Errorf("expected keyEsc, got %d", evt)
	}
}

func TestReadKeyRune(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	w.Write([]byte{'q'})
	w.Close()
	evt, ch := readKey(r)
	if evt != keyRune || ch != 'q' {
		t.Errorf("expected keyRune 'q', got %d %c", evt, ch)
	}
}

func TestReadKeyClosedPipe(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Close()
	evt, _ := readKey(r)
	r.Close()
	// Closed pipe should return keyEsc (not keyNone) to exit cleanly
	if evt != keyEsc {
		t.Errorf("expected keyEsc on closed pipe, got %d", evt)
	}
}

func TestReadKeySplitEscSeq_1_2(t *testing.T) {
	// Simulate ESC arriving alone, then '[' + 'A' shortly after (split read).
	escTimeout = 200 * time.Millisecond // widen for CI reliability
	defer func() { escTimeout = 50 * time.Millisecond }()

	r, w, _ := os.Pipe()
	defer r.Close()

	go func() {
		w.Write([]byte{0x1b})           // ESC alone
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte{'[', 'A'})       // rest of sequence
		w.Close()
	}()

	evt, _ := readKey(r)
	if evt != keyUp {
		t.Errorf("expected keyUp from split 1+2, got %d", evt)
	}
}

func TestReadKeySplitEscSeq_1_1_1(t *testing.T) {
	// Simulate ESC, then '[', then 'B' arriving as three separate reads.
	escTimeout = 200 * time.Millisecond
	defer func() { escTimeout = 50 * time.Millisecond }()

	r, w, _ := os.Pipe()
	defer r.Close()

	go func() {
		w.Write([]byte{0x1b})
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte{'['})
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte{'B'})
		w.Close()
	}()

	evt, _ := readKey(r)
	if evt != keyDown {
		t.Errorf("expected keyDown from split 1+1+1, got %d", evt)
	}
}

func TestReadKeySplitEscSeq_2_1(t *testing.T) {
	// Simulate ESC + '[' arriving together, then 'A' separately.
	escTimeout = 200 * time.Millisecond
	defer func() { escTimeout = 50 * time.Millisecond }()

	r, w, _ := os.Pipe()
	defer r.Close()

	go func() {
		w.Write([]byte{0x1b, '['})
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte{'A'})
		w.Close()
	}()

	evt, _ := readKey(r)
	if evt != keyUp {
		t.Errorf("expected keyUp from split 2+1, got %d", evt)
	}
}

func TestMenuItemActionInvalidThenQuit(t *testing.T) {
	// Type "d", then invalid "99", then menu re-displays, then quit
	in := strings.NewReader("d\n99\nq\n")
	out := &strings.Builder{}

	result, err := showLineMenu(in, out, MenuConfig{
		Title: "Sessions",
		Items: []MenuItem{
			{Key: "s1", Label: "session1"},
		},
		ExtraActions: []ExtraAction{
			{Key: "d", Label: "Detach", ID: "detach", ItemAction: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionQuit {
		t.Errorf("expected quit, got %+v", result)
	}
	if !strings.Contains(out.String(), "Invalid selection") {
		t.Error("expected invalid selection message")
	}
}
