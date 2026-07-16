package statusline

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const statusJSON = `{"model":{"id":"claude-opus-4-8","display_name":"Opus 4.8"}}`

func TestProfileName(t *testing.T) {
	cases := []struct{ dir, want string }{
		{"", "default"},
		{filepath.Join("home", ".claude"), "default"},
		{filepath.Join("home", ".claude-profiles", "acme"), "acme"},
	}
	for _, c := range cases {
		if got := ProfileName(c.dir); got != c.want {
			t.Errorf("ProfileName(%q) = %q, want %q", c.dir, got, c.want)
		}
	}
}

func TestRenderShowsProfileAndModel(t *testing.T) {
	var out bytes.Buffer
	dir := filepath.Join(t.TempDir(), "acme")
	if err := Render(strings.NewReader(statusJSON), &out, dir, false); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out.String()), "acme · Opus 4.8"; got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

func TestRenderToleratesMissingInput(t *testing.T) {
	var out bytes.Buffer
	if err := Render(strings.NewReader(""), &out, "", false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "default" {
		t.Fatalf("Render = %q, want %q", got, "default")
	}
}

func TestRenderColor(t *testing.T) {
	var out bytes.Buffer
	if err := Render(strings.NewReader(statusJSON), &out, "profiles/acme", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\x1b[1;36macme\x1b[0m") {
		t.Fatalf("expected colored profile name, got %q", out.String())
	}
}

func TestRenderChainsOriginalCommand(t *testing.T) {
	dir := t.TempDir()
	stash := `{"type":"command","command":"echo original-line"}`
	if err := os.WriteFile(filepath.Join(dir, StashFile), []byte(stash), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(strings.NewReader(statusJSON), &out, dir, false); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, filepath.Base(dir)+" · Opus 4.8 │ ") || !strings.HasSuffix(got, "original-line") {
		t.Fatalf("Render = %q, want prefix then chained output", got)
	}
}

func TestRenderIgnoresBrokenChain(t *testing.T) {
	dir := t.TempDir()
	stash := `{"type":"command","command":"exit 3"}`
	if err := os.WriteFile(filepath.Join(dir, StashFile), []byte(stash), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(strings.NewReader(statusJSON), &out, dir, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); strings.Contains(got, "│") {
		t.Fatalf("failing chained command must be dropped, got %q", got)
	}
}

func TestInstallFreshProfile(t *testing.T) {
	dir := t.TempDir()
	chained, err := Install(dir, "/opt/claude-profile")
	if err != nil {
		t.Fatal(err)
	}
	if chained {
		t.Fatal("fresh install reported a chained original")
	}
	settings := readSettingsMap(t, dir)
	var cfg struct{ Type, Command string }
	if err := json.Unmarshal(settings["statusLine"], &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Type != "command" || cfg.Command != `"/opt/claude-profile" statusline` {
		t.Fatalf("unexpected statusLine: %+v", cfg)
	}
}

func TestInstallPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	orig := `{"model":"opus","permissions":{"allow":["Bash(ls:*)"]}}`
	writeSettingsFile(t, dir, orig)
	if _, err := Install(dir, "/opt/claude-profile"); err != nil {
		t.Fatal(err)
	}
	settings := readSettingsMap(t, dir)
	if string(settings["model"]) != `"opus"` {
		t.Fatalf("model setting mangled: %s", settings["model"])
	}
	if string(settings["permissions"]) != `{"allow":["Bash(ls:*)"]}` {
		t.Fatalf("permissions mangled: %s", settings["permissions"])
	}
}

func TestInstallStashesForeignStatusLine(t *testing.T) {
	dir := t.TempDir()
	foreign := `{"type":"command","command":"~/.claude/statusline.sh","padding":0}`
	writeSettingsFile(t, dir, `{"statusLine":`+foreign+`}`)

	chained, err := Install(dir, "/opt/claude-profile")
	if err != nil {
		t.Fatal(err)
	}
	if !chained {
		t.Fatal("expected chained=true when a foreign statusLine exists")
	}
	stash := readFile(t, filepath.Join(dir, StashFile))
	if stash != foreign {
		t.Fatalf("stash = %s, want original entry byte-for-byte", stash)
	}
}

func TestInstallIsIdempotentAndRefreshesExe(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, "/old/claude-profile"); err != nil {
		t.Fatal(err)
	}
	chained, err := Install(dir, "/new/claude-profile")
	if err != nil {
		t.Fatal(err)
	}
	if chained {
		t.Fatal("reinstall must not treat our own entry as a foreign original")
	}
	if fileExists(filepath.Join(dir, StashFile)) {
		t.Fatal("reinstall created a stash of our own entry")
	}
	settings := readSettingsMap(t, dir)
	if !strings.Contains(string(settings["statusLine"]), "/new/claude-profile") {
		t.Fatalf("exe path not refreshed: %s", settings["statusLine"])
	}
}

func TestInstallRefusesInvalidSettings(t *testing.T) {
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{not json`)
	if _, err := Install(dir, "/opt/claude-profile"); err == nil {
		t.Fatal("expected an error on invalid settings.json")
	}
	if got := readFile(t, filepath.Join(dir, SettingsFile)); got != `{not json` {
		t.Fatal("invalid settings.json was modified")
	}
}

func TestUninstallRestoresOriginal(t *testing.T) {
	dir := t.TempDir()
	foreign := `{"type":"command","command":"my-statusline"}`
	writeSettingsFile(t, dir, `{"statusLine":`+foreign+`}`)
	if _, err := Install(dir, "/opt/claude-profile"); err != nil {
		t.Fatal(err)
	}

	restored, err := Uninstall(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !restored {
		t.Fatal("expected the original statusLine to be restored")
	}
	settings := readSettingsMap(t, dir)
	if string(settings["statusLine"]) != foreign {
		t.Fatalf("statusLine = %s, want restored original", settings["statusLine"])
	}
	if fileExists(filepath.Join(dir, StashFile)) {
		t.Fatal("stash file not removed after restore")
	}
}

func TestUninstallWithoutStashRemovesEntry(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, "/opt/claude-profile"); err != nil {
		t.Fatal(err)
	}
	restored, err := Uninstall(dir)
	if err != nil {
		t.Fatal(err)
	}
	if restored {
		t.Fatal("nothing to restore, yet restored=true")
	}
	settings := readSettingsMap(t, dir)
	if _, ok := settings["statusLine"]; ok {
		t.Fatal("statusLine entry still present after uninstall")
	}
}

func TestUninstallRefusesForeignStatusLine(t *testing.T) {
	dir := t.TempDir()
	writeSettingsFile(t, dir, `{"statusLine":{"type":"command","command":"my-statusline"}}`)
	if _, err := Uninstall(dir); err == nil {
		t.Fatal("expected an error uninstalling a foreign statusLine")
	}
}

func readSettingsMap(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	m, err := readSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func writeSettingsFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, SettingsFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
