package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Root: t.TempDir()}
}

func TestValidateName(t *testing.T) {
	valid := []string{"work", "Work-2", "a", "personal_dev", "9lives", strings.Repeat("a", 64)}
	invalid := []string{"", "-work", "_x", "wo rk", "wo/rk", "wo.rk", "../evil", "a;b", "a'b", strings.Repeat("a", 65)}

	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}

func TestCreateListRemove(t *testing.T) {
	s := newStore(t)

	if profiles, err := s.List(); err != nil || len(profiles) != 0 {
		t.Fatalf("List() on empty store = %v, %v; want empty, nil", profiles, err)
	}

	for _, name := range []string{"work", "personal"} {
		if _, err := s.Create(name); err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
	}

	if _, err := s.Create("work"); err == nil {
		t.Fatal("Create of existing profile should fail")
	}

	profiles, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || profiles[0].Name != "personal" || profiles[1].Name != "work" {
		t.Fatalf("List() = %v, want [personal work] sorted", profiles)
	}

	if err := s.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if s.Exists("work") {
		t.Fatal("profile still exists after Remove")
	}
	if err := s.Remove("work"); err == nil {
		t.Fatal("Remove of missing profile should fail")
	}
}

func TestCreateRejectsInvalidName(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create("../escape"); err == nil {
		t.Fatal("Create with path traversal name should fail")
	}
	if _, err := os.Stat(filepath.Join(s.Root, "..", "escape")); err == nil {
		t.Fatal("traversal directory was created")
	}
}

func TestCopyFrom(t *testing.T) {
	s := newStore(t)
	src := t.TempDir()

	// Shareable items.
	mustWrite(t, filepath.Join(src, "settings.json"), `{"theme":"dark"}`)
	mustWrite(t, filepath.Join(src, "CLAUDE.md"), "# rules")
	mustWrite(t, filepath.Join(src, "skills", "my-skill", "SKILL.md"), "skill")
	// Items that must never be copied.
	mustWrite(t, filepath.Join(src, ".credentials.json"), "SECRET")
	mustWrite(t, filepath.Join(src, "history.jsonl"), "old chats")

	p, err := s.Create("clone")
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.CopyFrom(src, filepath.Join(src, ".claude.json"), EverythingSelection())
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("CopyFrom copied %d items, want 3", n)
	}

	for _, want := range []string{"settings.json", "CLAUDE.md", filepath.Join("skills", "my-skill", "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(p.Dir, want)); err != nil {
			t.Errorf("expected %s to be copied: %v", want, err)
		}
	}
	for _, banned := range []string{".credentials.json", "history.jsonl"} {
		if _, err := os.Stat(filepath.Join(p.Dir, banned)); err == nil {
			t.Errorf("%s must never be copied", banned)
		}
	}
}

func TestCopyFromSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	s := newStore(t)
	src := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	mustWrite(t, secret, "token")
	if err := os.MkdirAll(filepath.Join(src, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(src, "skills", "link")); err != nil {
		t.Fatal(err)
	}

	p, err := s.Create("nolinks")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CopyFrom(src, filepath.Join(src, ".claude.json"), EverythingSelection()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "skills", "link")); err == nil {
		t.Fatal("symlink was copied; expected it to be skipped")
	}
}

func TestDefaultRootHonorsEnv(t *testing.T) {
	t.Setenv(EnvRoot, "/custom/root")
	root, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != "/custom/root" {
		t.Fatalf("DefaultRoot() = %q, want /custom/root", root)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
