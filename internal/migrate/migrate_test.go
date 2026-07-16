package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/statusline"
)

func testStore(t *testing.T, names ...string) *profile.Store {
	t.Helper()
	store := &profile.Store{Root: t.TempDir()}
	for _, n := range names {
		if _, err := store.Create(n); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func ctx() Context { return Context{Exe: "/opt/claude-profile"} }

func TestRegistryIsOrderedAndUnique(t *testing.T) {
	seen := map[int]bool{}
	last := 0
	for _, m := range All {
		if m.ID <= last {
			t.Fatalf("registry must be append-only in ID order; %d after %d", m.ID, last)
		}
		if seen[m.ID] {
			t.Fatalf("duplicate migration ID %d", m.ID)
		}
		seen[m.ID] = true
		last = m.ID
		if m.Name == "" || m.Summary == "" || m.Apply == nil {
			t.Fatalf("migration %d is incomplete", m.ID)
		}
	}
}

func TestRunAppliesStatuslineToOldProfiles(t *testing.T) {
	store := testStore(t, "old")
	reports, err := Run(store, ctx(), "0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != len(All) {
		t.Fatalf("got %d reports, want %d", len(reports), len(All))
	}
	if reports[0].Result != Applied || reports[0].Err != nil {
		t.Fatalf("statusline migration: %+v", reports[0])
	}
	installed, err := statusline.Installed(store.Dir("old"))
	if err != nil || !installed {
		t.Fatalf("status line not installed (installed=%v err=%v)", installed, err)
	}
}

func TestRunIsRecordedAndNeverRepeats(t *testing.T) {
	store := testStore(t, "old")
	if _, err := Run(store, ctx(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	// The user deliberately removes the feature afterwards.
	if _, err := statusline.Uninstall(store.Dir("old")); err != nil {
		t.Fatal(err)
	}
	reports, err := Run(store, ctx(), "0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("recorded migrations must never re-run, got %+v", reports)
	}
	if installed, _ := statusline.Installed(store.Dir("old")); installed {
		t.Fatal("a removed feature was forced back")
	}
}

func TestRunChainsForeignStatusline(t *testing.T) {
	store := testStore(t, "old")
	dir := store.Dir("old")
	if err := os.WriteFile(filepath.Join(dir, statusline.SettingsFile),
		[]byte(`{"statusLine":{"type":"command","command":"other-tool"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reports, err := Run(store, ctx(), "0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if reports[0].Result != Applied || !strings.Contains(reports[0].Detail, "kept") {
		t.Fatalf("foreign status line must be chained: %+v", reports[0])
	}
	if _, err := os.Stat(filepath.Join(dir, statusline.StashFile)); err != nil {
		t.Fatal("original status line was not stashed")
	}
}

func TestRunSkipsWhenAlreadyPresent(t *testing.T) {
	store := testStore(t, "old")
	if _, err := statusline.Install(store.Dir("old"), "/opt/claude-profile"); err != nil {
		t.Fatal(err)
	}
	reports, err := Run(store, ctx(), "0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if reports[0].Result != Skipped {
		t.Fatalf("expected skip, got %+v", reports[0])
	}
}

func TestStampAllBlocksMigrations(t *testing.T) {
	store := testStore(t, "fresh")
	p := profile.Profile{Name: "fresh", Dir: store.Dir("fresh")}
	if err := StampAll(p, "0.4.0"); err != nil {
		t.Fatal(err)
	}
	pend, err := Pending(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(pend) != 0 {
		t.Fatalf("stamped profile has pending migrations: %v", pend)
	}
	if AnyPending(store) {
		t.Fatal("AnyPending must be false after stamping")
	}
}

func TestStatus(t *testing.T) {
	store := testStore(t, "done", "todo")
	if err := StampAll(profile.Profile{Name: "done", Dir: store.Dir("done")}, "0.4.0"); err != nil {
		t.Fatal(err)
	}
	status, err := Status(store)
	if err != nil {
		t.Fatal(err)
	}
	if status["done"][All[0].ID] != Builtin {
		t.Fatalf("done: %v", status["done"])
	}
	if _, recorded := status["todo"][All[0].ID]; recorded {
		t.Fatalf("todo must be pending: %v", status["todo"])
	}
}

func TestMetaSurvivesUnknownFields(t *testing.T) {
	store := testStore(t, "p")
	dir := store.Dir("p")
	if err := os.WriteFile(filepath.Join(dir, MetaFile),
		[]byte(`{"schema":1,"future":"stuff","migrations":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p := profile.Profile{Name: "p", Dir: dir}
	if _, err := Pending(p); err != nil {
		t.Fatalf("meta with unknown fields must still parse: %v", err)
	}
}
