package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rc")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetLineCreatesFileAndBlock(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "sub", "rc") // parent dir does not exist yet
	if err := SetLine(rc, "alias claude-work=", "alias claude-work='cp run work'"); err != nil {
		t.Fatal(err)
	}
	got := read(t, rc)
	for _, want := range []string{BlockStart, "alias claude-work='cp run work'", BlockEnd} {
		if !strings.Contains(got, want) {
			t.Errorf("rc file missing %q:\n%s", want, got)
		}
	}
}

func TestSetLinePreservesUserContent(t *testing.T) {
	user := "export PATH=$PATH:/opt/bin\nalias ll='ls -la'\n"
	rc := tmpFile(t, user)

	if err := SetLine(rc, "alias claude-a=", "alias claude-a='cp run a'"); err != nil {
		t.Fatal(err)
	}
	if err := SetLine(rc, "alias claude-b=", "alias claude-b='cp run b'"); err != nil {
		t.Fatal(err)
	}

	got := read(t, rc)
	if !strings.HasPrefix(got, user) {
		t.Errorf("user content was modified:\n%s", got)
	}
	if strings.Count(got, BlockStart) != 1 || strings.Count(got, BlockEnd) != 1 {
		t.Errorf("expected exactly one managed block:\n%s", got)
	}
	if !strings.Contains(got, "claude-a") || !strings.Contains(got, "claude-b") {
		t.Errorf("expected both aliases present:\n%s", got)
	}
}

func TestSetLineIsIdempotent(t *testing.T) {
	rc := tmpFile(t, "")
	for i := 0; i < 3; i++ {
		if err := SetLine(rc, "alias claude-x=", "alias claude-x='cp run x'"); err != nil {
			t.Fatal(err)
		}
	}
	got := read(t, rc)
	if n := strings.Count(got, "alias claude-x="); n != 1 {
		t.Errorf("alias appears %d times, want 1:\n%s", n, got)
	}
}

func TestSetLineReplacesChangedLine(t *testing.T) {
	rc := tmpFile(t, "")
	if err := SetLine(rc, "alias claude-x=", "alias claude-x='old'"); err != nil {
		t.Fatal(err)
	}
	if err := SetLine(rc, "alias claude-x=", "alias claude-x='new'"); err != nil {
		t.Fatal(err)
	}
	got := read(t, rc)
	if strings.Contains(got, "'old'") || !strings.Contains(got, "'new'") {
		t.Errorf("line was not replaced:\n%s", got)
	}
}

func TestRemoveLine(t *testing.T) {
	rc := tmpFile(t, "# mine\n")
	_ = SetLine(rc, "alias claude-a=", "alias claude-a='cp run a'")
	_ = SetLine(rc, "alias claude-b=", "alias claude-b='cp run b'")

	if err := RemoveLine(rc, "alias claude-a="); err != nil {
		t.Fatal(err)
	}
	got := read(t, rc)
	if strings.Contains(got, "claude-a") {
		t.Errorf("claude-a still present:\n%s", got)
	}
	if !strings.Contains(got, "claude-b") {
		t.Errorf("claude-b was removed too:\n%s", got)
	}

	// Removing the last line removes the whole block.
	if err := RemoveLine(rc, "alias claude-b="); err != nil {
		t.Fatal(err)
	}
	got = read(t, rc)
	if strings.Contains(got, BlockStart) || strings.Contains(got, BlockEnd) {
		t.Errorf("empty block should be removed:\n%s", got)
	}
	if !strings.Contains(got, "# mine") {
		t.Errorf("user content lost:\n%s", got)
	}
}

func TestRemoveLineMissingFileIsNoop(t *testing.T) {
	if err := RemoveLine(filepath.Join(t.TempDir(), "nope"), "alias x="); err != nil {
		t.Fatalf("RemoveLine on missing file: %v", err)
	}
}

func TestHasLine(t *testing.T) {
	rc := tmpFile(t, "  alias claude='my custom thing'\n")
	got, err := HasLine(rc, "alias claude=")
	if err != nil || !got {
		t.Fatalf("HasLine = %v, %v; want true, nil", got, err)
	}
	got, err = HasLine(rc, "alias other=")
	if err != nil || got {
		t.Fatalf("HasLine = %v, %v; want false, nil", got, err)
	}
}

func TestShellLines(t *testing.T) {
	tests := []struct {
		sh   Shell
		want string
	}{
		{Bash{}, `alias claude-work='"/usr/local/bin/claude-profile" run work'`},
		{Zsh{}, `alias claude-work='"/usr/local/bin/claude-profile" run work'`},
		{PowerShell{}, "function claude-work { & '/usr/local/bin/claude-profile' run work @args }"},
	}
	for _, tt := range tests {
		got := tt.sh.AliasLine("work", "/usr/local/bin/claude-profile")
		if got != tt.want {
			t.Errorf("%s AliasLine = %q, want %q", tt.sh.Name(), got, tt.want)
		}
		if !strings.HasPrefix(got, tt.sh.AliasKey("work")) {
			t.Errorf("%s AliasLine does not start with its AliasKey", tt.sh.Name())
		}
		if !strings.HasPrefix(tt.sh.GuardLine("/x/cp"), tt.sh.GuardKey()) {
			t.Errorf("%s GuardLine does not start with its GuardKey", tt.sh.Name())
		}
	}
}

func TestAliasLinesSurviveSpacesInExePath(t *testing.T) {
	exe := "/Users/My Name/bin/claude-profile"
	for _, sh := range []Shell{Bash{}, Zsh{}, PowerShell{}} {
		for _, line := range []string{
			sh.AliasLine("work", exe),
			sh.GuardLine(exe),
			sh.SelfAliasLine("claudep", exe),
		} {
			if !strings.Contains(line, `"`+exe+`"`) && !strings.Contains(line, "'"+exe+"'") {
				t.Errorf("%s line leaves a spaced path unquoted: %s", sh.Name(), line)
			}
		}
	}
}

func TestDetectHonorsRcOverride(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "custom-rc")
	t.Setenv("CLAUDE_PROFILE_RC", rc)
	shells := Detect()
	if len(shells) != 1 {
		t.Fatalf("Detect() with override returned %d shells, want 1", len(shells))
	}
	file, err := shells[0].StartupFile()
	if err != nil || file != rc {
		t.Fatalf("StartupFile() = %q, %v; want %q", file, err, rc)
	}
}
