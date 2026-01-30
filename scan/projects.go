// Package scan builds shell commands to discover git repositories using
// platform-specific tools, and parses the results into project entries.
package scan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// ScanResult represents a discovered project directory.
type ScanResult struct {
	Path string // absolute path to project (parent of .git)
	Name string // directory name
}

// BuildScanChainCommand returns a shell command that tries discovery tools in
// order, using the first one that produces non-empty output:
// mdfind -> plocate -> locate -> fd -> find.
// Each tool is only tried if present on the system (checked via command -v).
// The find fallback always runs as it's universally available.
func BuildScanChainCommand(homeDir string) string {
	mdfind := buildMdfindCommand(homeDir)
	locate := buildLocateCommand(homeDir)
	fd := buildFdCommand(homeDir)
	find := buildFindCommand(homeDir)

	// Each tool is guarded by `command -v`. If it exists, run it and capture
	// output. If the output is non-empty, print it and exit. Otherwise fall
	// through to the next tool.
	tryTool := func(guard, cmd string) string {
		return fmt.Sprintf(
			`if command -v %s >/dev/null 2>&1; then _out=$(%s 2>/dev/null); if [ -n "$_out" ]; then printf '%%s\n' "$_out"; exit 0; fi; fi`,
			guard, cmd,
		)
	}

	parts := []string{
		tryTool("mdfind", mdfind),
		tryTool("plocate", locate), // locate command tries plocate first via the command itself
		tryTool("locate", locate),
		tryTool("fd", fd),
		find, // find is always available as final fallback
	}

	return strings.Join(parts, "; ")
}

// buildMdfindCommand returns the mdfind shell command for discovering .git
// entries under homeDir.
func buildMdfindCommand(homeDir string) string {
	return fmt.Sprintf(`mdfind "kMDItemFSName == '.git'" -onlyin %s`, shellutil.Quote(homeDir))
}

// buildLocateCommand returns a locate/plocate shell command for discovering
// .git entries under homeDir.
func buildLocateCommand(homeDir string) string {
	escaped := regexp.QuoteMeta(homeDir)
	return fmt.Sprintf(`plocate --regex '%s/.*/.git$' 2>/dev/null || locate --regex '%s/.*/.git$'`, escaped, escaped)
}

// buildFdCommand returns an fd shell command for discovering .git entries under
// homeDir. Matches both directories (.git/) and files (.git for submodules).
func buildFdCommand(homeDir string) string {
	return fmt.Sprintf(`fd -H -t d -t f '^\\.git$' %s --max-depth 4`, shellutil.Quote(homeDir))
}

// buildFindCommand returns a find shell command for discovering .git entries
// under homeDir, matching both directories and files (.git can be a file in
// submodules). Common noise directories are excluded.
func buildFindCommand(homeDir string) string {
	return fmt.Sprintf(
		`find %s -maxdepth 4 -name .git \( -type d -o -type f \) -not -path '*/node_modules/*' -not -path '*/Library/*' -not -path '*/.cache/*' -not -path '*/.Trash/*'`,
		shellutil.Quote(homeDir),
	)
}

// ParseScanResults parses newline-delimited paths to .git entries and returns
// deduplicated ScanResults with parent directories.
func ParseScanResults(output string) []ScanResult {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	seen := make(map[string]bool)
	var results []ScanResult

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip the .git suffix to get the project directory.
		parent := filepath.Dir(line)
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

// nonAlphanumHyphen matches characters that are not alphanumeric or hyphens.
var nonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]`)

// leadingTrailingHyphens matches leading or trailing hyphens.
var leadingTrailingHyphens = regexp.MustCompile(`^-+|-+$`)

// DeriveProjectKey takes an absolute path, extracts the directory name,
// lowercases it, replaces underscores and spaces with hyphens, removes
// non-alphanumeric/hyphen characters, and trims leading/trailing hyphens.
func DeriveProjectKey(path string) string {
	name := filepath.Base(path)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	name = nonAlphanumHyphen.ReplaceAllString(name, "")
	name = leadingTrailingHyphens.ReplaceAllString(name, "")
	return name
}
