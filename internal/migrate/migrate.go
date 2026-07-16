// Package migrate brings profiles created by older versions up to date with
// features new profiles get automatically. See docs/updates.md.
//
// The registry is append-only: IDs never change meaning and entries are
// never removed. Every profile records which migrations have run in a meta
// file; a recorded migration is never re-applied, so features a user has
// deliberately removed stay removed. Apply functions must be idempotent and
// must never touch credentials, history, or foreign configuration.
package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/victorRadu/claude-profile/internal/profile"
)

// MetaFile records applied migrations inside each profile directory. The
// leading dot keeps it visually apart from Claude Code's own files.
const MetaFile = ".claude-profile-meta.json"

// Result classifies what applying a migration did.
type Result string

const (
	// Applied means the migration changed the profile.
	Applied Result = "applied"
	// Skipped means the feature was already present (or user-modified), so
	// nothing was changed; recorded all the same, never revisited.
	Skipped Result = "skipped"
	// Builtin marks migrations stamped at profile creation, where the
	// feature was built in from the start.
	Builtin Result = "builtin"
)

// Context carries what Apply functions may need.
type Context struct {
	// Exe is the claude-profile binary path, for migrations that write it
	// into a profile's configuration (e.g. the status line command).
	Exe string
}

// Migration is one append-only registry entry.
type Migration struct {
	ID      int
	Name    string
	Summary string // one line, printed when applied: "status line shows profile and model"
	// Apply must be idempotent. detail may add context ("existing status
	// line kept"); it is shown dimmed after the summary.
	Apply func(p profile.Profile, ctx Context) (result Result, detail string, err error)
}

type record struct {
	Name        string `json:"name"`
	Result      Result `json:"result"`
	AppliedUnix int64  `json:"appliedUnix"`
	Binary      string `json:"binary"`
}

type meta struct {
	Schema     int               `json:"schema"`
	Migrations map[string]record `json:"migrations"`
}

// Report is the outcome of one migration on one profile.
type Report struct {
	Profile   string
	Migration Migration
	Result    Result
	Detail    string
	Err       error
}

// now is a variable for tests.
var now = time.Now

// Pending returns the registry entries not yet recorded for p, in ID order.
func Pending(p profile.Profile) ([]Migration, error) {
	m, err := readMeta(p.Dir)
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, mig := range All {
		if _, done := m.Migrations[strconv.Itoa(mig.ID)]; !done {
			out = append(out, mig)
		}
	}
	return out, nil
}

// AnyPending reports whether any profile in store has pending migrations.
func AnyPending(store *profile.Store) bool {
	profiles, err := store.List()
	if err != nil {
		return false
	}
	for _, p := range profiles {
		if pend, err := Pending(p); err == nil && len(pend) > 0 {
			return true
		}
	}
	return false
}

// Run applies every pending migration to every profile, in ID order.
// A failure is reported and retried on the next run (only success is
// recorded); it never blocks other migrations or profiles.
func Run(store *profile.Store, ctx Context, version string) ([]Report, error) {
	profiles, err := store.List()
	if err != nil {
		return nil, err
	}
	var reports []Report
	for _, p := range profiles {
		pend, err := Pending(p)
		if err != nil {
			reports = append(reports, Report{Profile: p.Name, Err: err})
			continue
		}
		for _, mig := range pend {
			res, detail, err := mig.Apply(p, ctx)
			rep := Report{Profile: p.Name, Migration: mig, Result: res, Detail: detail, Err: err}
			if err == nil {
				if err := record1(p.Dir, mig, res, version); err != nil {
					rep.Err = fmt.Errorf("applied, but could not record it: %w", err)
				}
			}
			reports = append(reports, rep)
		}
	}
	return reports, nil
}

// StampAll marks every registered migration as built in. Called by create,
// because a new profile already has every current feature.
func StampAll(p profile.Profile, version string) error {
	for _, mig := range All {
		if err := record1(p.Dir, mig, Builtin, version); err != nil {
			return err
		}
	}
	return nil
}

// Status returns, per profile, each migration's recorded result ("" when
// pending), for `migrate --status`.
func Status(store *profile.Store) (map[string]map[int]Result, error) {
	profiles, err := store.List()
	if err != nil {
		return nil, err
	}
	out := map[string]map[int]Result{}
	for _, p := range profiles {
		m, err := readMeta(p.Dir)
		if err != nil {
			return nil, fmt.Errorf("profile %s: %w", p.Name, err)
		}
		row := map[int]Result{}
		for _, mig := range All {
			if rec, ok := m.Migrations[strconv.Itoa(mig.ID)]; ok {
				row[mig.ID] = rec.Result
			}
		}
		out[p.Name] = row
	}
	return out, nil
}

func record1(dir string, mig Migration, res Result, version string) error {
	m, err := readMeta(dir)
	if err != nil {
		return err
	}
	m.Migrations[strconv.Itoa(mig.ID)] = record{
		Name:        mig.Name,
		Result:      res,
		AppliedUnix: now().Unix(),
		Binary:      version,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, MetaFile), append(data, '\n'), 0o600)
}

func readMeta(dir string) (meta, error) {
	m := meta{Schema: 1, Migrations: map[string]record{}}
	data, err := os.ReadFile(filepath.Join(dir, MetaFile))
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("%s is not valid JSON: %w", MetaFile, err)
	}
	if m.Migrations == nil {
		m.Migrations = map[string]record{}
	}
	return m, nil
}
