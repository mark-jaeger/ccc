package shellutil

import "testing"

func TestQuoteSimple(t *testing.T) {
	got := Quote("hello")
	if got != "'hello'" {
		t.Errorf("Quote(hello) = %q, want %q", got, "'hello'")
	}
}

func TestQuoteWithSpaces(t *testing.T) {
	got := Quote("/Users/mark/My Project")
	if got != "'/Users/mark/My Project'" {
		t.Errorf("Quote(path with space) = %q, want %q", got, "'/Users/mark/My Project'")
	}
}

func TestQuoteWithSingleQuotes(t *testing.T) {
	got := Quote("it's")
	want := "'it'\\''s'"
	if got != want {
		t.Errorf("Quote(it's) = %q, want %q", got, want)
	}
}

func TestQuoteEmpty(t *testing.T) {
	got := Quote("")
	if got != "''" {
		t.Errorf("Quote() = %q, want %q", got, "''")
	}
}
