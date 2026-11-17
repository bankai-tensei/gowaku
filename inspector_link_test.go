package gowaku

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Inspector exists only with -tags gowakudebug: a release build must not
// link a byte of it, nor the probe of core (docs/ADR/0073-inspector.md). The
// test builds examples/counter both ways and reads the symbols.
func TestInspector_NotLinkedWithoutTheTag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program twice")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command:", err)
	}
	dir := t.TempDir()
	symbols := func(tags ...string) string {
		t.Helper()
		out := filepath.Join(dir, "counter"+strings.Join(tags, "-")+".exe")
		args := []string{"build", "-o", out}
		if len(tags) > 0 {
			args = append(args, "-tags", strings.Join(tags, ","))
		}
		args = append(args, "./examples/counter")
		if msg, err := exec.Command(goTool, args...).CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, msg)
		}
		nm, err := exec.Command(goTool, "tool", "nm", out).Output()
		if err != nil {
			t.Fatalf("go tool nm: %v", err)
		}
		return string(nm)
	}
	const pkg = "github.com/bankai-tensei/gowaku/internal/inspect"
	release := symbols()
	for _, name := range []string{pkg, "SetProbe", "runtime/metrics."} {
		if line := lineWith(release, name); line != "" {
			t.Errorf("a build without the tag links %s:\n%s", name, line)
		}
	}
	debug := symbols("gowakudebug")
	// SetProbe itself is inlined, so only the packages prove the build.
	for _, name := range []string{pkg + ".", "runtime/metrics."} {
		if lineWith(debug, name) == "" {
			t.Errorf("a build with -tags gowakudebug must link %s; the test reads the wrong symbols", name)
		}
	}
}

// lineWith returns the first line of s that holds sub, or "".
func lineWith(s, sub string) string {
	for line := range strings.Lines(s) {
		if strings.Contains(line, sub) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
