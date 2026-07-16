package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/shell"
)

func TestAliasSetRenameRemove(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	rc := filepath.Join(t.TempDir(), "rc")
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	// set
	if code := app.Run([]string{"alias", "claudep"}); code != 0 {
		t.Fatal("alias set failed")
	}
	got := readFile(t, rc)
	if !strings.Contains(got, `alias claudep='"`+app.Exe+`"'`) || !strings.Contains(got, shell.SelfAliasTag) {
		t.Fatalf("rc missing tagged claudep alias:\n%s", got)
	}

	// rename replaces, never duplicates
	if code := app.Run([]string{"alias", "cpf"}); code != 0 {
		t.Fatal("alias rename failed")
	}
	got = readFile(t, rc)
	if strings.Contains(got, "claudep") {
		t.Fatalf("old alias still present after rename:\n%s", got)
	}
	if !strings.Contains(got, "alias cpf='") {
		t.Fatalf("new alias missing:\n%s", got)
	}
	if strings.Count(got, shell.SelfAliasTag) != 1 {
		t.Fatalf("expected exactly one tagged line:\n%s", got)
	}

	// show
	out.Reset()
	if code := app.Run([]string{"alias"}); code != 0 {
		t.Fatal("alias show failed")
	}
	if !strings.Contains(out.String(), "cpf") {
		t.Fatalf("show should print the current alias:\n%s", out)
	}

	// remove
	if code := app.Run([]string{"alias", "--remove"}); code != 0 {
		t.Fatal("alias remove failed")
	}
	if strings.Contains(readFile(t, rc), shell.SelfAliasTag) {
		t.Fatal("tagged line still present after remove")
	}

	// show after remove
	out.Reset()
	app.Run([]string{"alias"})
	if !strings.Contains(out.String(), "No short alias") {
		t.Fatalf("expected 'No short alias':\n%s", out)
	}
}

func TestAliasCoexistsWithProfileAliases(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	rc := filepath.Join(t.TempDir(), "rc")
	t.Setenv("CLAUDE_PROFILE_RC", rc)

	app.Run([]string{"create", "go"}) // installs claude-go alias
	app.Run([]string{"alias", "claudep"})
	app.Run([]string{"alias", "cpf"}) // rename must not touch claude-go

	got := readFile(t, rc)
	if !strings.Contains(got, "claude-go") {
		t.Fatalf("profile alias lost:\n%s", got)
	}
	if !strings.Contains(got, "alias cpf='") {
		t.Fatalf("self alias missing:\n%s", got)
	}
}

func TestAliasRejectsClaudeAndInvalidNames(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	if code := app.Run([]string{"alias", "claude"}); code == 0 {
		t.Fatal("alias 'claude' must be rejected")
	}
	if !strings.Contains(errBuf.String(), "shadow") {
		t.Fatalf("error should explain the shadowing problem: %s", errBuf)
	}
	if code := app.Run([]string{"alias", "bad name"}); code == 0 {
		t.Fatal("invalid alias name must be rejected")
	}
}

func TestTaggedLineHelpers(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "rc")
	if err := shell.SetTaggedLine(rc, shell.SelfAliasTag, "alias a='x'  "+shell.SelfAliasTag); err != nil {
		t.Fatal(err)
	}
	if err := shell.SetTaggedLine(rc, shell.SelfAliasTag, "alias b='x'  "+shell.SelfAliasTag); err != nil {
		t.Fatal(err)
	}
	line, ok, err := shell.FindTaggedLine(rc, shell.SelfAliasTag)
	if err != nil || !ok || !strings.Contains(line, "alias b=") {
		t.Fatalf("FindTaggedLine = %q, %v, %v; want the replaced line", line, ok, err)
	}
	if err := shell.RemoveTaggedLine(rc, shell.SelfAliasTag); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := shell.FindTaggedLine(rc, shell.SelfAliasTag); ok {
		t.Fatal("tagged line still found after removal")
	}
}
