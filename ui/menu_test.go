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
			{Key: "a", Label: "Add host", ID: "add"},
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

func TestMenuRemoveDeclined(t *testing.T) {
	// User types "r", "1", "n" (decline) → menu re-displays → then "q"
	in := strings.NewReader("r\n1\nn\nq\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title:      "Sessions",
		Items:      []MenuItem{{Key: "s1", Label: "session1"}},
		ShowRemove: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionQuit {
		t.Errorf("expected quit after decline, got %+v", result)
	}
	// Menu should have been displayed twice (initial + after decline)
	if strings.Count(out.String(), "Sessions") < 2 {
		t.Error("expected menu to re-display after declined remove")
	}
}

func TestMenuRemoveInvalidSelection(t *testing.T) {
	// User types "r", "99" (invalid) → menu re-displays → then "q"
	in := strings.NewReader("r\n99\nq\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title:      "Sessions",
		Items:      []MenuItem{{Key: "s1", Label: "session1"}},
		ShowRemove: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionQuit {
		t.Errorf("expected quit after invalid remove selection, got %+v", result)
	}
	if !strings.Contains(out.String(), "Invalid selection") {
		t.Error("expected 'Invalid selection' message")
	}
}

func TestPromptReturnsInput(t *testing.T) {
	in := strings.NewReader("hello world\n")
	out := &bytes.Buffer{}

	answer, err := Prompt(in, out, "Name")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", answer)
	}
	if !strings.Contains(out.String(), "Name:") {
		t.Error("expected prompt text in output")
	}
}

func TestPromptEOF(t *testing.T) {
	in := strings.NewReader("")
	out := &bytes.Buffer{}

	answer, err := Prompt(in, out, "Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "" {
		t.Errorf("expected empty string on EOF, got %q", answer)
	}
}

func TestConfirmYes(t *testing.T) {
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	ok, err := Confirm(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for 'y'")
	}
}

func TestConfirmNo(t *testing.T) {
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}

	ok, err := Confirm(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false for 'n'")
	}
}

func TestMenuExtraActionsIDField(t *testing.T) {
	in := strings.NewReader("s\n")
	out := &bytes.Buffer{}

	result, err := ShowMenu(in, out, MenuConfig{
		Title: "Projects",
		Items: []MenuItem{{Key: "p1", Label: "project1"}},
		ExtraActions: []ExtraAction{
			{Key: "s", Label: "Scan", ID: "scan"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionExtra || result.ExtraKey != "scan" {
		t.Errorf("expected extra scan, got %+v", result)
	}
}
