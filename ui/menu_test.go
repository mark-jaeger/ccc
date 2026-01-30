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
