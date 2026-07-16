package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Repo is the GitHub repository updates come from.
const Repo = "victorRadu/claude-profile"

// APIBase is a variable so tests can point the checker at a local server.
var APIBase = "https://api.github.com"

// CheckInterval is how long the cached check result stays fresh.
const CheckInterval = 24 * time.Hour

// EnvNoCheck disables the background version check and the update notice.
const EnvNoCheck = "CLAUDE_PROFILE_NO_UPDATE_CHECK"

// stateDirName holds claude-profile's own state inside the profiles root.
// A leading dot guarantees it can never collide with a profile name.
const stateDirName = ".state"

const (
	cacheFile    = "update.json"
	noticeFile   = "notice.json"
	migratedFile = "migrated-version"
)

// State reads and writes the update-check state for one profiles root.
// The zero value is not usable; Now is required so tests control time.
type State struct {
	Root string // profiles root, e.g. ~/.claude-profiles
	Now  func() time.Time
}

type cacheState struct {
	Schema        int    `json:"schema"`
	LastCheckUnix int64  `json:"lastCheckUnix"`
	LatestVersion string `json:"latestVersion"`
}

type noticeState struct {
	Schema         int    `json:"schema"`
	LastNoticeUnix int64  `json:"lastNoticeUnix"`
	Version        string `json:"version"`
}

func (c State) stateDir() string { return filepath.Join(c.Root, stateDirName) }

// Enabled reports whether checking is allowed at all for this build and
// environment. Interactivity is the caller's concern.
func Enabled(version string) bool {
	return os.Getenv(EnvNoCheck) == "" && os.Getenv("CI") == "" && !IsDevBuild(version)
}

// Stale reports whether the cache is due for a background refresh.
func (c State) Stale() bool {
	var st cacheState
	if !c.read(cacheFile, &st) {
		return true
	}
	return c.Now().Sub(time.Unix(st.LastCheckUnix, 0)) >= CheckInterval
}

// Notice returns the one-line update notice to show, or "" when there is
// nothing to say. A returned notice is throttled: it repeats for the same
// version at most once per CheckInterval.
func (c State) Notice(current string) string {
	var st cacheState
	if !c.read(cacheFile, &st) || !Newer(st.LatestVersion, current) {
		return ""
	}
	var n noticeState
	if c.read(noticeFile, &n) && n.Version == st.LatestVersion &&
		c.Now().Sub(time.Unix(n.LastNoticeUnix, 0)) < CheckInterval {
		return ""
	}
	c.write(noticeFile, noticeState{Schema: 1, LastNoticeUnix: c.Now().Unix(), Version: st.LatestVersion})
	return fmt.Sprintf("↑ claude-profile %s is available (you have %s) — run: claude-profile update",
		Canonical(st.LatestVersion), Canonical(current))
}

// Refresh queries the releases API and rewrites the cache. The timestamp
// advances even on failure, so an offline machine probes at most once per
// interval. Run in the detached background process, never the foreground.
func (c State) Refresh(client *http.Client) error {
	st := cacheState{Schema: 1, LastCheckUnix: c.Now().Unix()}
	var old cacheState
	if c.read(cacheFile, &old) {
		st.LatestVersion = old.LatestVersion // keep last known on failure
	}
	latest, err := LatestRelease(client)
	if err == nil {
		st.LatestVersion = latest.Version
	}
	c.write(cacheFile, st)
	return err
}

// Release describes the latest published release.
type Release struct {
	Version string            // canonical, no leading v
	Assets  map[string]string // asset name → download URL
}

// LatestRelease queries the GitHub API for the newest release.
func LatestRelease(client *http.Client) (Release, error) {
	resp, err := client.Get(APIBase + "/repos/" + Repo + "/releases/latest")
	if err != nil {
		return Release{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release query returned %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("parsing release response: %w", err)
	}
	if body.TagName == "" {
		return Release{}, errors.New("release response has no tag name")
	}
	rel := Release{Version: Canonical(body.TagName), Assets: map[string]string{}}
	for _, a := range body.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// MigratedVersion returns the binary version whose migration pass last
// completed, or "" when none is recorded.
func (c State) MigratedVersion() string {
	data, err := os.ReadFile(filepath.Join(c.stateDir(), migratedFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetMigratedVersion records a completed migration pass for version v.
func (c State) SetMigratedVersion(v string) {
	if err := os.MkdirAll(c.stateDir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(c.stateDir(), migratedFile), []byte(v+"\n"), 0o600)
}

// read unmarshals a state file; false means absent or unreadable.
func (c State) read(name string, v any) bool {
	data, err := os.ReadFile(filepath.Join(c.stateDir(), name))
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// write persists a state file; state is advisory, so errors are dropped —
// the worst case is an extra check or notice later.
func (c State) write(name string, v any) {
	if err := os.MkdirAll(c.stateDir(), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(c.stateDir(), name), data, 0o600)
}
