package tailscale

import (
	"testing"
)

func TestParseTailscaleStatus(t *testing.T) {
	input := `100.64.0.1    macbook-m1     mark@        macOS   -
100.64.0.5    server-lab     mark@        linux   -
100.64.0.10   phone          mark@        iOS     -`

	hosts := ParseTailscaleStatus(input)

	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}

	tests := []struct {
		index int
		ip    string
		name  string
		user  string
		os    string
	}{
		{0, "100.64.0.1", "macbook-m1", "mark", "macOS"},
		{1, "100.64.0.5", "server-lab", "mark", "linux"},
		{2, "100.64.0.10", "phone", "mark", "iOS"},
	}

	for _, tt := range tests {
		h := hosts[tt.index]
		if h.IP != tt.ip {
			t.Errorf("host[%d].IP = %q, want %q", tt.index, h.IP, tt.ip)
		}
		if h.Name != tt.name {
			t.Errorf("host[%d].Name = %q, want %q", tt.index, h.Name, tt.name)
		}
		if h.User != tt.user {
			t.Errorf("host[%d].User = %q, want %q", tt.index, h.User, tt.user)
		}
		if h.OS != tt.os {
			t.Errorf("host[%d].OS = %q, want %q", tt.index, h.OS, tt.os)
		}
	}
}

func TestParseTailscaleStatusEmpty(t *testing.T) {
	hosts := ParseTailscaleStatus("")
	if hosts != nil {
		t.Errorf("expected nil for empty input, got %v", hosts)
	}
}

func TestParseTailscaleStatusWithSelfLine(t *testing.T) {
	input := `100.64.0.1    macbook-m1     mark@        macOS   -
100.64.0.5    server-lab     mark@        linux   active; relay "tok", tx 1234 rx 5678`

	hosts := ParseTailscaleStatus(input)

	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}

	if hosts[0].IP != "100.64.0.1" {
		t.Errorf("host[0].IP = %q, want %q", hosts[0].IP, "100.64.0.1")
	}
	if hosts[0].Name != "macbook-m1" {
		t.Errorf("host[0].Name = %q, want %q", hosts[0].Name, "macbook-m1")
	}
	if hosts[0].User != "mark" {
		t.Errorf("host[0].User = %q, want %q", hosts[0].User, "mark")
	}
	if hosts[0].OS != "macOS" {
		t.Errorf("host[0].OS = %q, want %q", hosts[0].OS, "macOS")
	}

	if hosts[1].IP != "100.64.0.5" {
		t.Errorf("host[1].IP = %q, want %q", hosts[1].IP, "100.64.0.5")
	}
	if hosts[1].Name != "server-lab" {
		t.Errorf("host[1].Name = %q, want %q", hosts[1].Name, "server-lab")
	}
	if hosts[1].User != "mark" {
		t.Errorf("host[1].User = %q, want %q", hosts[1].User, "mark")
	}
	if hosts[1].OS != "linux" {
		t.Errorf("host[1].OS = %q, want %q", hosts[1].OS, "linux")
	}
}
