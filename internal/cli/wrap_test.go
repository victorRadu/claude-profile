package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// launchedAt records calls to the LaunchAt hook.
type launchedAt struct {
	path      string
	configDir string
	args      []string
	called    bool
}

func hookLaunchAt(app *App) *launchedAt {
	l := &launchedAt{}
	app.LaunchAt = func(path, configDir string, args []string) error {
		*l = launchedAt{path: path, configDir: configDir, args: args, called: true}
		return nil
	}
	return l
}

// fakeClaude drops an executable named claude into a temp dir and puts
// that dir (alone) on PATH.
func fakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		name = "claude.bat"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return path
}

func TestWrapInstallAndUninstall(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	rc := filepath.Join(t.TempDir(), "rc")
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	if code := app.Run([]string{"wrap", "install"}); code != 0 {
		t.Fatalf("wrap install failed: %s", out)
	}
	shim := app.shimPath()
	content := readFile(t, shim)
	if !strings.Contains(content, "wrap-exec") || !strings.Contains(content, app.Exe) {
		t.Fatalf("shim must call '%s wrap-exec':\n%s", app.Exe, content)
	}
	if !strings.Contains(readFile(t, rc), app.shimDir()) {
		t.Fatalf("rc missing PATH line:\n%s", readFile(t, rc))
	}

	if code := app.Run([]string{"wrap", "uninstall"}); code != 0 {
		t.Fatal("wrap uninstall failed")
	}
	if fileExists(shim) {
		t.Fatal("shim still present after uninstall")
	}
	if strings.Contains(readFile(t, rc), app.shimDir()) {
		t.Fatal("PATH line still present after uninstall")
	}
}

func TestWrapShimDirHiddenFromList(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.Run([]string{"wrap", "install"})
	app.Run([]string{"create", "real", "--no-alias"})

	out.Reset()
	app.Run([]string{"list"})
	if strings.Contains(out.String(), ".bin") {
		t.Fatalf(".bin must never appear as a profile:\n%s", out)
	}
}

func TestWrapExecBoundDirectory(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	realPath := fakeClaude(t)
	l := hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "go", "--no-alias"})
	app.Run([]string{"link", "go"})

	errBuf.Reset()
	if code := app.Run([]string{"wrap-exec", "--continue"}); code != 0 {
		t.Fatalf("wrap-exec failed: %s", errBuf)
	}
	if !l.called || l.path != realPath {
		t.Fatalf("launched %q, want real claude %q", l.path, realPath)
	}
	if l.configDir != app.Store.Dir("go") {
		t.Fatalf("configDir = %q, want go profile", l.configDir)
	}
	if len(l.args) != 1 || l.args[0] != "--continue" {
		t.Fatalf("args = %v, want [--continue]", l.args)
	}
	if !strings.Contains(errBuf.String(), "profile: go") {
		t.Fatalf("banner missing:\n%s", errBuf)
	}
}

func TestWrapExecUnboundNonInteractiveIsTransparent(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	realPath := fakeClaude(t)
	l := hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "go", "--no-alias"})

	if code := app.Run([]string{"wrap-exec", "-p", "hello"}); code != 0 {
		t.Fatalf("wrap-exec failed: %s", errBuf)
	}
	if l.path != realPath {
		t.Fatalf("launched %q, want real claude", l.path)
	}
	if l.configDir != "" {
		t.Fatalf("passthrough must not set a config dir, got %q", l.configDir)
	}
	if len(l.args) != 2 || l.args[0] != "-p" || l.args[1] != "hello" {
		t.Fatalf("args = %v, want [-p hello] unchanged", l.args)
	}
	if strings.Contains(errBuf.String(), "profile:") {
		t.Fatalf("passthrough must not print a profile banner:\n%s", errBuf)
	}
}

func TestWrapExecUnboundInteractiveShowsPicker(t *testing.T) {
	app, out, _, _ := newTestApp(t, "1\n")
	realPath := fakeClaude(t)
	l := hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "go", "--no-alias"})
	app.Interactive = true

	if code := app.Run([]string{"wrap-exec"}); code != 0 {
		t.Fatal("wrap-exec failed")
	}
	if !strings.Contains(out.String(), "1) go") {
		t.Fatalf("picker not shown:\n%s", out)
	}
	if l.path != realPath || l.configDir != app.Store.Dir("go") {
		t.Fatalf("picker launch = %q %q, want real claude with go profile", l.path, l.configDir)
	}
}

func TestWrapExecNoProfilesFallsThrough(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	fakeClaude(t)
	l := hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Interactive = true

	if code := app.Run([]string{"wrap-exec"}); code != 0 {
		t.Fatal("wrap-exec failed")
	}
	if !l.called || l.configDir != "" {
		t.Fatal("with no profiles, wrap-exec must pass through to stock claude")
	}
}

func TestWrapExecStaleMarkerFailsLoudly(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	fakeClaude(t)
	l := hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "old", "--no-alias"})
	app.Run([]string{"link", "old"})
	app.Run([]string{"remove", "old", "--force"})

	if code := app.Run([]string{"wrap-exec"}); code == 0 {
		t.Fatal("stale marker must fail, not silently pass through")
	}
	if l.called {
		t.Fatal("claude must not be launched with a stale marker")
	}
	if !strings.Contains(errBuf.String(), `"old"`) {
		t.Fatalf("error should name the profile: %s", errBuf)
	}
}

func TestFindRealClaudeSkipsShimDir(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	realPath := fakeClaude(t) // PATH = realDir

	// Put a decoy shim claude in the shim dir and prepend it to PATH.
	if err := os.MkdirAll(app.shimDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.shimPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", app.shimDir()+string(os.PathListSeparator)+os.Getenv("PATH"))

	found, err := findRealClaude(app.shimDir())
	if err != nil {
		t.Fatal(err)
	}
	if found != realPath {
		t.Fatalf("findRealClaude = %q, want %q (must skip the shim)", found, realPath)
	}
}

func TestFindRealClaudeSkipsSymlinkedShimDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	app, _, _, _ := newTestApp(t, "")
	realPath := fakeClaude(t) // PATH = realDir

	// Shim dir reachable via a symlinked alias in PATH: skipping only the
	// literal shim path would find the shim and recurse forever.
	if err := os.MkdirAll(app.shimDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.shimPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked-bin")
	if err := os.Symlink(app.shimDir(), linked); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", linked+string(os.PathListSeparator)+os.Getenv("PATH"))

	found, err := findRealClaude(app.shimDir())
	if err != nil {
		t.Fatal(err)
	}
	if found != realPath {
		t.Fatalf("findRealClaude = %q, want %q (symlinked shim dir must be skipped)", found, realPath)
	}
}

func TestWrapStatusWarnsOnStaleShim(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if err := os.MkdirAll(app.shimDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "#!/bin/sh\nexec \"/moved/away/claude-profile\" wrap-exec \"$@\"\n"
	if err := os.WriteFile(app.shimPath(), []byte(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"wrap", "status"}); code != 0 {
		t.Fatal("wrap status failed")
	}
	if !strings.Contains(out.String(), "different claude-profile binary") {
		t.Fatalf("status should warn about a stale shim:\n%s", out)
	}
}

func TestWrapExecNoRealClaude(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	l := hookLaunchAt(app)
	t.Setenv("PATH", t.TempDir()) // empty dir: no claude anywhere
	app.WorkDir = t.TempDir()

	if code := app.Run([]string{"wrap-exec"}); code == 0 {
		t.Fatal("wrap-exec without a real claude must fail")
	}
	if l.called {
		t.Fatal("nothing should be launched")
	}
	if !strings.Contains(errBuf.String(), "Claude Code") {
		t.Fatalf("error should mention Claude Code: %s", errBuf)
	}
}

func TestWrapStatusNotInstalled(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"wrap", "status"}); code != 0 {
		t.Fatal("wrap status failed")
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Fatalf("expected 'not installed':\n%s", out)
	}
}

func TestWrapExecBannerMentionsProfile(t *testing.T) {
	// The wrapper's banner is the certainty guarantee; make sure the
	// resolution source is included.
	app, _, errBuf, _ := newTestApp(t, "")
	fakeClaude(t)
	hookLaunchAt(app)
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "go", "--no-alias"})
	app.Run([]string{"link", "go"})
	errBuf.Reset()
	app.Run([]string{"wrap-exec"})
	if !strings.Contains(errBuf.String(), profile.MarkerName) {
		t.Fatalf("banner must say the marker resolved the profile:\n%s", errBuf)
	}
}
