// Package statusline renders and installs a Claude Code status line that
// always shows the active profile and the current model.
//
// Claude Code supports exactly one statusLine command in settings.json.
// To coexist with status lines from other tools, Install never discards an
// existing command: it stashes the original configuration next to
// settings.json and the renderer appends that command's output after the
// "profile · model" prefix. Uninstall restores the stashed original.
package statusline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SettingsFile is Claude Code's per-config-dir settings file.
const SettingsFile = "settings.json"

// StashFile holds the statusLine configuration that was in place before
// Install, so the renderer can chain it and Uninstall can restore it.
const StashFile = "statusline.original.json"

// chainTimeout bounds the chained original command, so a slow or hung
// third-party status line can never freeze ours.
const chainTimeout = 3 * time.Second

// maxInput bounds the status JSON read from stdin.
const maxInput = 1 << 20

// statusInput is the subset of Claude Code's status-line stdin JSON we use.
type statusInput struct {
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
}

// lineConfig is the subset of a statusLine settings entry we inspect.
type lineConfig struct {
	Command string `json:"command"`
}

// ProfileName derives the name shown in the status line from the
// CLAUDE_CONFIG_DIR the session was launched with. An unset variable or the
// standard ~/.claude directory both mean the default (non-profile) setup.
func ProfileName(configDir string) string {
	if configDir == "" {
		return "default"
	}
	base := filepath.Base(configDir)
	if base == ".claude" {
		return "default"
	}
	return base
}

// Render reads Claude Code's status JSON from r and writes a single line to
// w: the profile name and model, followed by the output of the stashed
// original status line command, if one exists in configDir.
func Render(r io.Reader, w io.Writer, configDir string, color bool) error {
	input, _ := io.ReadAll(io.LimitReader(r, maxInput))
	var st statusInput
	_ = json.Unmarshal(input, &st) // tolerate empty or malformed input

	model := st.Model.DisplayName
	if model == "" {
		model = st.Model.ID
	}
	line := paint(color, "1;36", ProfileName(configDir))
	if model != "" {
		line += paint(color, "2", " · ") + model
	}
	if orig := chainedOutput(configDir, input); orig != "" {
		line += paint(color, "2", " │ ") + orig
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func paint(color bool, code, s string) string {
	if !color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// chainedOutput runs the stashed original status line command with the same
// stdin JSON and returns the first line of its output. Any failure yields
// "" — a broken chained command must never break the status line itself.
func chainedOutput(configDir string, input []byte) string {
	if configDir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(configDir, StashFile))
	if err != nil {
		return ""
	}
	var cfg lineConfig
	if json.Unmarshal(raw, &cfg) != nil || strings.TrimSpace(cfg.Command) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), chainTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cfg.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cfg.Command)
	}
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}

// Command returns the statusLine command string that invokes exe. The path
// is quoted so paths with spaces survive sh, cmd and PowerShell alike.
func Command(exe string) string {
	return `"` + exe + `" statusline`
}

// IsOurs reports whether a statusLine command string invokes claude-profile.
func IsOurs(command string) bool {
	trimmed := strings.TrimSpace(command)
	return strings.HasSuffix(trimmed, " statusline") && strings.Contains(trimmed, "claude-profile")
}

// Installed reports whether configDir's settings.json statusLine is ours.
func Installed(configDir string) (bool, error) {
	settings, err := readSettings(configDir)
	if err != nil {
		return false, err
	}
	raw, ok := settings["statusLine"]
	if !ok {
		return false, nil
	}
	var cfg lineConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return false, nil
	}
	return IsOurs(cfg.Command), nil
}

// Install points configDir's settings.json statusLine at exe. An existing
// foreign statusLine is stashed (never discarded) so the renderer chains it;
// the returned chained flag reports whether such an original is in place.
// Installing over our own entry just refreshes the exe path.
func Install(configDir, exe string) (chained bool, err error) {
	settings, err := readSettings(configDir)
	if err != nil {
		return false, err
	}
	if raw, ok := settings["statusLine"]; ok {
		var cfg lineConfig
		_ = json.Unmarshal(raw, &cfg) // unrecognized shapes are stashed as-is
		if !IsOurs(cfg.Command) {
			if err := os.WriteFile(filepath.Join(configDir, StashFile), raw, 0o600); err != nil {
				return false, fmt.Errorf("saving original status line: %w", err)
			}
		}
	}
	entry, err := json.Marshal(map[string]any{"type": "command", "command": Command(exe)})
	if err != nil {
		return false, err
	}
	settings["statusLine"] = entry
	if err := writeSettings(configDir, settings); err != nil {
		return false, err
	}
	_, statErr := os.Stat(filepath.Join(configDir, StashFile))
	return statErr == nil, nil
}

// Uninstall removes our statusLine from configDir's settings.json,
// restoring the stashed original if one exists. The returned restored flag
// reports whether an original was put back.
func Uninstall(configDir string) (restored bool, err error) {
	settings, err := readSettings(configDir)
	if err != nil {
		return false, err
	}
	raw, ok := settings["statusLine"]
	if !ok {
		return false, errors.New("no statusLine configured")
	}
	var cfg lineConfig
	if json.Unmarshal(raw, &cfg) != nil || !IsOurs(cfg.Command) {
		return false, errors.New("the configured statusLine is not managed by claude-profile — leaving it untouched")
	}
	stash := filepath.Join(configDir, StashFile)
	if orig, err := os.ReadFile(stash); err == nil {
		settings["statusLine"] = json.RawMessage(orig)
		restored = true
	} else {
		delete(settings, "statusLine")
	}
	if err := writeSettings(configDir, settings); err != nil {
		return false, err
	}
	if restored {
		if err := os.Remove(stash); err != nil {
			return true, fmt.Errorf("status line restored, but could not remove %s: %w", StashFile, err)
		}
	}
	return restored, nil
}

// readSettings parses settings.json into raw entries, so every key we do
// not understand survives untouched in content (writing re-indents, but
// never alters values). A missing file is an empty map.
func readSettings(configDir string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(filepath.Join(configDir, SettingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", SettingsFile, err)
	}
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON, refusing to modify it: %w", SettingsFile, err)
	}
	return settings, nil
}

func writeSettings(configDir string, settings map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(configDir, SettingsFile), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", SettingsFile, err)
	}
	return nil
}
