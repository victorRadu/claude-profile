package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/profile"
)

func TestLinkStatusLaunchFlow(t *testing.T) {
	app, out, errBuf, l := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "acme", "--no-alias"})

	// link
	out.Reset()
	if code := app.Run([]string{"link", "acme"}); code != 0 {
		t.Fatalf("link failed: %s", errBuf)
	}
	if !fileExists(filepath.Join(app.WorkDir, profile.MarkerName)) {
		t.Fatal("marker file not written")
	}

	// status
	out.Reset()
	if code := app.Run([]string{"status"}); code != 0 {
		t.Fatal("status failed")
	}
	if !strings.Contains(out.String(), `"acme"`) || !strings.Contains(out.String(), profile.MarkerName) {
		t.Fatalf("status should name the profile and the marker:\n%s", out)
	}

	// bare `run` resolves the marker without any prompt
	errBuf.Reset()
	if code := app.Run([]string{"run", "--continue"}); code != 0 {
		t.Fatalf("run failed: %s", errBuf)
	}
	if !l.called || l.configDir != app.Store.Dir("acme") {
		t.Fatalf("run used %q, want acme dir", l.configDir)
	}
	if len(l.args) != 1 || l.args[0] != "--continue" {
		t.Fatalf("run args = %v, want [--continue]", l.args)
	}
	banner := errBuf.String()
	if !strings.Contains(banner, "profile: acme") || !strings.Contains(banner, profile.MarkerName) {
		t.Fatalf("banner must announce profile and source:\n%q", banner)
	}
}

func TestRunWalksUpToMarker(t *testing.T) {
	app, _, _, l := newTestApp(t, "")
	root := t.TempDir()
	app.WorkDir = root
	app.Run([]string{"create", "acme", "--no-alias"})
	app.Run([]string{"link", "acme"})

	nested := filepath.Join(root, "src", "deep")
	writeFile(t, filepath.Join(nested, "keep"), "")
	app.WorkDir = nested

	if code := app.Run([]string{"run"}); code != 0 {
		t.Fatal("bare run from nested dir failed")
	}
	if l.configDir != app.Store.Dir("acme") {
		t.Fatalf("nested run used %q, want acme", l.configDir)
	}
}

func TestLinkRequiresExistingProfile(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	if code := app.Run([]string{"link", "ghost"}); code == 0 {
		t.Fatal("link to missing profile should fail")
	}
	if !strings.Contains(errBuf.String(), "does not exist") {
		t.Fatalf("unexpected error: %s", errBuf)
	}
	if fileExists(filepath.Join(app.WorkDir, profile.MarkerName)) {
		t.Fatal("marker must not be written for missing profile")
	}
}

func TestRunUnboundNonInteractiveFails(t *testing.T) {
	app, _, errBuf, l := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "acme", "--no-alias"})

	if code := app.Run([]string{"run"}); code == 0 {
		t.Fatal("unbound non-interactive run must fail, never guess")
	}
	if l.called {
		t.Fatal("Launch called without explicit resolution")
	}
	if !strings.Contains(errBuf.String(), "link") {
		t.Fatalf("error should point at link/run: %s", errBuf)
	}
}

func TestRunUnboundInteractiveShowsPicker(t *testing.T) {
	app, out, errBuf, l := newTestApp(t, "1\n")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "acme", "--no-alias"})
	app.Interactive = true

	if code := app.Run([]string{"run"}); code != 0 {
		t.Fatalf("run failed: %s", errBuf)
	}
	if !strings.Contains(out.String(), "1) acme") {
		t.Fatalf("picker not shown:\n%s", out)
	}
	if l.configDir != app.Store.Dir("acme") {
		t.Fatal("picker choice not launched")
	}
	if !strings.Contains(errBuf.String(), "via interactive picker") {
		t.Fatalf("banner should say picker:\n%s", errBuf)
	}
}

func TestRunStaleMarkerFails(t *testing.T) {
	app, _, errBuf, l := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "old", "--no-alias"})
	app.Run([]string{"link", "old"})
	app.Run([]string{"remove", "old", "--force"})

	if code := app.Run([]string{"run"}); code == 0 {
		t.Fatal("run with stale marker must fail loudly")
	}
	if l.called {
		t.Fatal("Launch called for missing profile")
	}
	if !strings.Contains(errBuf.String(), `"old"`) {
		t.Fatalf("error should name the stale profile: %s", errBuf)
	}
}

func TestRunSeparatorSkipsNameParsing(t *testing.T) {
	app, _, _, l := newTestApp(t, "")
	root := t.TempDir()
	app.WorkDir = root
	app.Run([]string{"create", "acme", "--no-alias"})
	app.Run([]string{"link", "acme"})

	// `run -- acme` must treat "acme" as a claude argument, not a profile.
	if code := app.Run([]string{"run", "--", "acme", "-p", "hi"}); code != 0 {
		t.Fatal("run -- failed")
	}
	if l.configDir != app.Store.Dir("acme") {
		t.Fatal("marker resolution not used after --")
	}
	if len(l.args) != 3 || l.args[0] != "acme" || l.args[2] != "hi" {
		t.Fatalf("claude args = %v, want [acme -p hi]", l.args)
	}
}

func TestRunRejectsInvalidFirstArg(t *testing.T) {
	app, _, errBuf, l := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	if code := app.Run([]string{"run", "explain this repo"}); code == 0 {
		t.Fatal("invalid first arg must fail, never guess")
	}
	if l.called {
		t.Fatal("Launch must not be called")
	}
	if !strings.Contains(errBuf.String(), "--") {
		t.Fatalf("error should suggest the -- separator: %s", errBuf)
	}
}

func TestSplitRunArgs(t *testing.T) {
	tests := []struct {
		in      []string
		name    string
		args    []string
		wantErr bool
	}{
		{nil, "", nil, false},
		{[]string{"work"}, "work", nil, false},
		{[]string{"work", "--continue"}, "work", []string{"--continue"}, false},
		{[]string{"work", "--", "-p", "x"}, "work", []string{"-p", "x"}, false},
		{[]string{"--"}, "", nil, false},
		{[]string{"--", "hello"}, "", []string{"hello"}, false},
		{[]string{"-p", "x"}, "", []string{"-p", "x"}, false},
		{[]string{"not a name"}, "", nil, true},
	}
	for _, tt := range tests {
		name, args, err := splitRunArgs(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("splitRunArgs(%v) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if name != tt.name || len(args) != len(tt.args) {
			t.Errorf("splitRunArgs(%v) = %q, %v; want %q, %v", tt.in, name, args, tt.name, tt.args)
			continue
		}
		for i := range args {
			if args[i] != tt.args[i] {
				t.Errorf("splitRunArgs(%v) args = %v, want %v", tt.in, args, tt.args)
			}
		}
	}
}

func TestUnlink(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	app.Run([]string{"create", "acme", "--no-alias"})
	app.Run([]string{"link", "acme"})

	if code := app.Run([]string{"unlink"}); code != 0 {
		t.Fatal("unlink failed")
	}
	if fileExists(filepath.Join(app.WorkDir, profile.MarkerName)) {
		t.Fatal("marker still present")
	}

	// unlink again: no-op, still success
	out.Reset()
	if code := app.Run([]string{"unlink"}); code != 0 {
		t.Fatal("second unlink should be a no-op success")
	}
	if !strings.Contains(out.String(), "nothing to do") {
		t.Fatalf("expected no-op message:\n%s", out)
	}
}

func TestUnlinkPointsAtParentMarker(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	root := t.TempDir()
	app.WorkDir = root
	app.Run([]string{"create", "acme", "--no-alias"})
	app.Run([]string{"link", "acme"})

	nested := filepath.Join(root, "sub")
	writeFile(t, filepath.Join(nested, "keep"), "")
	app.WorkDir = nested

	if code := app.Run([]string{"unlink"}); code == 0 {
		t.Fatal("unlink should fail and point at the parent marker")
	}
	if !strings.Contains(errBuf.String(), "one exists in") {
		t.Fatalf("expected pointer to parent marker: %s", errBuf)
	}
}

func TestRunPrintsBanner(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	app.Run([]string{"create", "work", "--no-alias"})
	errBuf.Reset()
	if code := app.Run([]string{"run", "work"}); code != 0 {
		t.Fatal("run failed")
	}
	if !strings.Contains(errBuf.String(), "profile: work") || !strings.Contains(errBuf.String(), "explicit") {
		t.Fatalf("run banner missing:\n%q", errBuf.String())
	}
}

func TestStatusUnbound(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.WorkDir = t.TempDir()
	if code := app.Run([]string{"status"}); code != 0 {
		t.Fatal("status failed")
	}
	if !strings.Contains(out.String(), "no "+profile.MarkerName) {
		t.Fatalf("status should say unbound:\n%s", out)
	}
}
