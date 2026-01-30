package tailscale

import (
	"fmt"
	"os/exec"
	"strings"
)

// TailscaleHost represents a peer discovered via the Tailscale CLI.
type TailscaleHost struct {
	IP   string
	Name string
	User string
	OS   string
}

// IsAvailable reports whether the tailscale CLI is in PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("tailscale")
	return err == nil
}

// DiscoverHosts runs tailscale status --peers and returns discovered hosts.
func DiscoverHosts() ([]TailscaleHost, error) {
	out, err := exec.Command("tailscale", "status", "--peers").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status failed: %w", err)
	}
	return ParseTailscaleStatus(string(out)), nil
}

// ParseTailscaleStatus parses the space-separated output of tailscale status.
// Each line is expected to have at least 4 whitespace-separated fields:
// IP, hostname, user (trailing @ trimmed), and OS. Lines with fewer than
// 4 fields are skipped. Empty input returns nil.
func ParseTailscaleStatus(output string) []TailscaleHost {
	if output == "" {
		return nil
	}

	var hosts []TailscaleHost
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		hosts = append(hosts, TailscaleHost{
			IP:   fields[0],
			Name: fields[1],
			User: strings.TrimRight(fields[2], "@"),
			OS:   fields[3],
		})
	}
	return hosts
}
