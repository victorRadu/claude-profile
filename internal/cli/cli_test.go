package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// launched records calls to the Launch hook.
type launched struct {
	configDir string
	args      []string
	called    bool
}

func newTestApp(t *testing.T, stdin string) (*App, *bytes.Buffer, *bytes.Buffer, *launched) {
	t.Helper()
	rc := filepath.Join(t.TempDir(), "rc")
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	var out, errBuf bytes.Buffer
	l := &launched{}
	app := &App{
		Store:   &profile.Store{Root: t.TempDir()},
		Stdin:   strings.NewReader(stdin),
		Stdout:  &out,
		Stderr:  &errBuf,
		Version: "test",
		Exe:     "/bin/claude-profile",
		Launch: func(dir string, args []string) error {
			*l = launched{configDir: dir, args: args, called: true}
			return nil
		},
		Interactive: false,
	}
	return app, &out, &errBuf, l
}

func TestCreateListRemoveFlow(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")

	if code := app.Run([]string{"create", "work"}); code != 0 {
		t.Fatalf("create exited %d: %s", code, out)
	}
	if !app.Store.Exists("work") {
		t.Fatal("profile directory not created")
	}

	out.Reset()
	if code := app.Run([]string{"list"}); code != 0 {
		t.Fatal("list failed")
	}
	if !strings.Contains(out.String(), "work") {
		t.Fatalf("list output missing profile: %s", out)
	}

	if code := app.Run([]string{"remove", "work", "--force"}); code != 0 {
		t.Fatal("remove failed")
	}
	if app.Store.Exists("work") {
		t.Fatal("profile still exists after remove")
	}
}

func TestCreateInstallsAlias(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	rc := t.TempDir() + "/myrc"
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	if code := app.Run([]string{"create", "dev"}); code != 0 {
		t.Fatal("create failed")
	}
	got := readFile(t, rc)
	want := `alias claude-dev='"/bin/claude-profile" run dev'`
	if !strings.Contains(got, want) {
		t.Fatalf("rc missing %q:\n%s", want, got)
	}
}

func TestCreateRejectsBadName(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "../evil"}); code == 0 {
		t.Fatal("create with bad name should fail")
	}
	if !strings.Contains(errBuf.String(), "invalid profile name") {
		t.Fatalf("unexpected error output: %s", errBuf)
	}
}

func TestCreateFromCopiesSettings(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "src", "--no-alias"}); code != 0 {
		t.Fatal("create src failed")
	}
	writeFile(t, filepath.Join(app.Store.Dir("src"), "settings.json"), `{}`)
	writeFile(t, filepath.Join(app.Store.Dir("src"), ".credentials.json"), "SECRET")

	out.Reset()
	if code := app.Run([]string{"create", "dst", "--from", "src", "--no-alias"}); code != 0 {
		t.Fatalf("create dst failed: %s", out)
	}
	if !fileExists(filepath.Join(app.Store.Dir("dst"), "settings.json")) {
		t.Fatal("settings.json not copied")
	}
	if fileExists(filepath.Join(app.Store.Dir("dst"), ".credentials.json")) {
		t.Fatal("credentials copied — must never happen")
	}
}

func TestRunLaunchesWithConfigDir(t *testing.T) {
	app, _, _, l := newTestApp(t, "")
	app.Run([]string{"create", "work", "--no-alias"})

	if code := app.Run([]string{"run", "work", "--continue"}); code != 0 {
		t.Fatal("run failed")
	}
	if !l.called {
		t.Fatal("Launch was not called")
	}
	if l.configDir != app.Store.Dir("work") {
		t.Fatalf("Launch config dir = %q, want %q", l.configDir, app.Store.Dir("work"))
	}
	if len(l.args) != 1 || l.args[0] != "--continue" {
		t.Fatalf("Launch args = %v, want [--continue]", l.args)
	}
}

func TestRunUnknownProfile(t *testing.T) {
	app, _, errBuf, l := newTestApp(t, "")
	if code := app.Run([]string{"run", "ghost"}); code == 0 {
		t.Fatal("run of unknown profile should fail")
	}
	if l.called {
		t.Fatal("Launch must not be called for unknown profile")
	}
	if !strings.Contains(errBuf.String(), "does not exist") {
		t.Fatalf("unexpected error: %s", errBuf)
	}
}

func TestRemoveNonInteractiveNeedsForce(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	app.Run([]string{"create", "keep", "--no-alias"})

	if code := app.Run([]string{"remove", "keep"}); code == 0 {
		t.Fatal("remove without --force should fail when non-interactive")
	}
	if !strings.Contains(errBuf.String(), "--force") {
		t.Fatalf("error should mention --force: %s", errBuf)
	}
	if !app.Store.Exists("keep") {
		t.Fatal("profile was deleted without confirmation")
	}
}

func TestRemoveDeletesAlias(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	rc := t.TempDir() + "/rc"
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	app.Run([]string{"create", "tmp"})
	if !strings.Contains(readFile(t, rc), "claude-tmp") {
		t.Fatal("alias not installed")
	}
	app.Run([]string{"remove", "tmp", "--force"})
	if strings.Contains(readFile(t, rc), "claude-tmp") {
		t.Fatal("alias not removed")
	}
}

func TestRunPickerChoice(t *testing.T) {
	app, _, _, l := newTestApp(t, "2\n")
	app.WorkDir = t.TempDir() // unbound directory
	app.Run([]string{"create", "alpha", "--no-alias"})
	app.Run([]string{"create", "beta", "--no-alias"})
	app.Interactive = true // only the picker itself should read stdin

	if code := app.Run([]string{"run"}); code != 0 {
		t.Fatal("run picker failed")
	}
	if l.configDir != app.Store.Dir("beta") {
		t.Fatalf("picker launched %q, want beta", l.configDir)
	}
}

func TestRunPickerCancelled(t *testing.T) {
	app, _, _, l := newTestApp(t, "\n")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "alpha", "--no-alias"})
	app.Interactive = true
	if code := app.Run([]string{"run"}); code != 0 {
		t.Fatal("cancelled picker should exit 0")
	}
	if l.called {
		t.Fatal("Launch called after cancel")
	}
}

// A first token that is not a known subcommand is handed to run, which
// treats it as a profile name — so a typo surfaces as a missing profile,
// not an "unknown command".
func TestUnknownCommandFallsThroughToRun(t *testing.T) {
	app, _, errBuf, l := newTestApp(t, "")
	if code := app.Run([]string{"frobnicate"}); code != 1 {
		t.Fatalf("unknown token exit = %d, want 1", code)
	}
	if l.called {
		t.Fatal("Launch must not be called for a non-existent profile")
	}
	if !strings.Contains(errBuf.String(), "does not exist") {
		t.Fatalf("unexpected output: %s", errBuf)
	}
}

// `claude-profile <profile> [args]` launches that profile without the
// explicit `run` verb, and everything after the name reaches claude.
func TestProfileNameFallsThroughToRun(t *testing.T) {
	app, _, _, l := newTestApp(t, "")
	app.Run([]string{"create", "work", "--no-alias"})

	if code := app.Run([]string{"work", "--continue"}); code != 0 {
		t.Fatal("passthrough run failed")
	}
	if l.configDir != app.Store.Dir("work") {
		t.Fatalf("launched %q, want work", l.configDir)
	}
	if len(l.args) != 1 || l.args[0] != "--continue" {
		t.Fatalf("Launch args = %v, want [--continue]", l.args)
	}
}

// A leading flag means "no profile name": resolve the binding, then pass
// the flags through to claude.
func TestLeadingFlagFallsThroughToRun(t *testing.T) {
	app, _, _, l := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "solo", "--no-alias"})
	app.Run([]string{"link", "solo"}) // bind the dir so resolution needs no picker

	if code := app.Run([]string{"--resume"}); code != 0 {
		t.Fatalf("leading-flag passthrough failed")
	}
	if !l.called {
		t.Fatal("Launch was not called")
	}
	if len(l.args) != 1 || l.args[0] != "--resume" {
		t.Fatalf("Launch args = %v, want [--resume]", l.args)
	}
}

func TestVersionAndHelp(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.Run([]string{"version"})
	if !strings.Contains(out.String(), "claude-profile test") {
		t.Fatalf("version output: %s", out)
	}
	out.Reset()
	if code := app.Run([]string{"help"}); code != 0 || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help should print usage, got code %d: %s", code, out)
	}
}

// Bare invocation is no longer help — it resolves the binding, then the
// picker (never a silent default).
func TestBareInvocationRunsPicker(t *testing.T) {
	app, _, _, l := newTestApp(t, "2\n")
	app.WorkDir = t.TempDir() // unbound directory
	app.Run([]string{"create", "alpha", "--no-alias"})
	app.Run([]string{"create", "beta", "--no-alias"})
	app.Interactive = true

	if code := app.Run(nil); code != 0 {
		t.Fatalf("bare invocation should run the picker, got code %d", code)
	}
	if l.configDir != app.Store.Dir("beta") {
		t.Fatalf("picker launched %q, want beta", l.configDir)
	}
}
