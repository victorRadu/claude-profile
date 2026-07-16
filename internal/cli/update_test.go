package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/migrate"
	"github.com/victorRadu/claude-profile/internal/statusline"
	"github.com/victorRadu/claude-profile/internal/update"
)

// fakeAPI serves a minimal latest-release response and points the update
// package at it for the duration of the test.
func fakeAPI(t *testing.T, tag string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[]}`, tag)
	}))
	old := update.APIBase
	update.APIBase = srv.URL
	t.Cleanup(func() { update.APIBase = old; srv.Close() })
}

// releasedApp returns a test app that looks like an installed release.
func releasedApp(t *testing.T, version string) (*App, *strings.Builder, *strings.Builder) {
	t.Helper()
	app, _, _, _ := newTestApp(t, "")
	var out, errBuf strings.Builder
	app.Stdout, app.Stderr = &out, &errBuf
	app.Version = version
	app.Interactive = true
	t.Setenv("CI", "")
	t.Setenv(update.EnvNoCheck, "")
	return app, &out, &errBuf
}

func TestUpdateCheckReportsNewVersion(t *testing.T) {
	fakeAPI(t, "v9.9.9")
	app, out, errBuf := releasedApp(t, "0.3.1")
	if code := app.Run([]string{"update", "--check"}); code != 1 {
		t.Fatalf("expected exit 1 when outdated, got %d (%s%s)", code, out, errBuf)
	}
	if !strings.Contains(out.String(), "9.9.9") || strings.Contains(errBuf.String(), "Error:") {
		t.Fatalf("unexpected output: out=%q err=%q", out, errBuf)
	}
}

func TestUpdateCheckUpToDate(t *testing.T) {
	fakeAPI(t, "v0.3.1")
	app, out, _ := releasedApp(t, "0.3.1")
	if code := app.Run([]string{"update", "--check"}); code != 0 {
		t.Fatalf("expected exit 0 when current, got %d", code)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("output: %q", out)
	}
}

func TestUpdateRefusesDevBuild(t *testing.T) {
	fakeAPI(t, "v9.9.9")
	app, _, errBuf, _ := newTestApp(t, "") // Version "test" = dev build
	if code := app.Run([]string{"update"}); code != 1 {
		t.Fatal("dev build must refuse to self-update")
	}
	if !strings.Contains(errBuf.String(), "development build") {
		t.Fatalf("error: %q", errBuf)
	}
}

func TestRefreshCacheThenNotice(t *testing.T) {
	fakeAPI(t, "v9.9.9")
	app, _, errBuf := releasedApp(t, "0.3.1")

	if code := app.Run([]string{"update", "--refresh-cache"}); code != 0 {
		t.Fatal("refresh-cache failed")
	}
	// Next ordinary command must show the cached notice on stderr.
	if code := app.Run([]string{"list"}); code != 0 {
		t.Fatal("list failed")
	}
	if !strings.Contains(errBuf.String(), "9.9.9 is available") {
		t.Fatalf("expected update notice on stderr, got %q", errBuf)
	}
}

func TestNoticeSuppressedByEnv(t *testing.T) {
	fakeAPI(t, "v9.9.9")
	app, _, errBuf := releasedApp(t, "0.3.1")
	if code := app.Run([]string{"update", "--refresh-cache"}); code != 0 {
		t.Fatal("refresh-cache failed")
	}
	t.Setenv(update.EnvNoCheck, "1")
	if code := app.Run([]string{"list"}); code != 0 {
		t.Fatal("list failed")
	}
	if strings.Contains(errBuf.String(), "is available") {
		t.Fatalf("notice must be suppressed, got %q", errBuf)
	}
}

func TestAutoMigrateAnnouncedOnNewVersion(t *testing.T) {
	app, _, errBuf := releasedApp(t, "0.4.0")
	t.Setenv(update.EnvNoCheck, "1") // isolate: no background check
	// A profile that predates migrations: created directly in the store,
	// like a profile from an old binary.
	if _, err := app.Store.Create("legacy"); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"list"}); code != 0 {
		t.Fatalf("list failed: %s", errBuf)
	}
	if !strings.Contains(errBuf.String(), "bringing existing profiles up to date") ||
		!strings.Contains(errBuf.String(), "status line") {
		t.Fatalf("auto-migration not announced: %q", errBuf)
	}
	installed, err := statusline.Installed(app.Store.Dir("legacy"))
	if err != nil || !installed {
		t.Fatal("migration did not install the status line")
	}
	// Second run: nothing pending, no announcement.
	errBuf.Reset()
	if code := app.Run([]string{"list"}); code != 0 {
		t.Fatal("second list failed")
	}
	if strings.Contains(errBuf.String(), "bringing existing profiles") {
		t.Fatalf("auto-migration must run once per version, got %q", errBuf)
	}
}

func TestMigrateCommandAndStatus(t *testing.T) {
	app, out, _ := releasedApp(t, "0.4.0")
	t.Setenv(update.EnvNoCheck, "1")
	app.Interactive = false // keep create non-interactive (no copy prompt)
	if code := app.Run([]string{"create", "fresh", "--no-alias"}); code != 0 {
		t.Fatal("create failed")
	}
	app.Interactive = true
	if _, err := app.Store.Create("legacy"); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if code := app.Run([]string{"migrate", "--status"}); code != 0 {
		t.Fatal("migrate --status failed")
	}
	if !strings.Contains(out.String(), "pending") || !strings.Contains(out.String(), "builtin") {
		t.Fatalf("status must show pending and builtin: %q", out)
	}

	out.Reset()
	if code := app.Run([]string{"migrate"}); code != 0 {
		t.Fatal("migrate failed")
	}
	if !strings.Contains(out.String(), "legacy") || !strings.Contains(out.String(), "status line") {
		t.Fatalf("migrate output: %q", out)
	}

	out.Reset()
	if code := app.Run([]string{"migrate"}); code != 0 {
		t.Fatal("second migrate failed")
	}
	if !strings.Contains(out.String(), "All profiles are up to date.") {
		t.Fatalf("second migrate output: %q", out)
	}
}

func TestCreateStampsMigrations(t *testing.T) {
	app, _, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"create", "work", "--no-alias"}); code != 0 {
		t.Fatal("create failed")
	}
	if !fileExists(filepath.Join(app.Store.Dir("work"), migrate.MetaFile)) {
		t.Fatal("create did not stamp the migration meta file")
	}
	if migrate.AnyPending(app.Store) {
		t.Fatal("a freshly created profile must have no pending migrations")
	}
}
