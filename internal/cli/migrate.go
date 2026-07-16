package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/victorRadu/claude-profile/internal/migrate"
	"github.com/victorRadu/claude-profile/internal/update"
)

// migrateCmd implements `claude-profile migrate [--status]`.
func (a *App) migrateCmd(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	status := fs.Bool("status", false, "show each profile's migration state without changing anything")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: claude-profile migrate [--status]")
	}
	if *status {
		return a.migrationStatus()
	}
	return a.applyMigrations(a.Stdout, true)
}

// applyMigrations runs all pending migrations and prints what happened.
// verbose additionally reports skips and the all-clear.
func (a *App) applyMigrations(w io.Writer, verbose bool) error {
	reports, err := migrate.Run(a.Store, migrate.Context{Exe: a.Exe}, a.Version)
	if err != nil {
		return err
	}
	st := a.Style
	changed, failed := 0, 0
	for _, r := range reports {
		switch {
		case r.Err != nil:
			failed++
			fmt.Fprintf(a.Stderr, "%s %s: %s failed: %v %s\n",
				st.yellow("!"), r.Profile, r.Migration.Name, r.Err, st.dim("(will retry next run)"))
		case r.Result == migrate.Applied:
			changed++
			detail := ""
			if r.Detail != "" {
				detail = " " + st.dim("("+r.Detail+")")
			}
			fmt.Fprintf(w, "%s %s: %s%s\n", st.green("✓"), st.bold(st.cyan(r.Profile)), r.Migration.Summary, detail)
		case verbose:
			fmt.Fprintf(w, "%s %s: %s %s\n", st.dim("·"), r.Profile, r.Migration.Name, st.dim("already present — left as is"))
		}
	}
	if failed == 0 {
		a.updateState().SetMigratedVersion(a.Version)
	}
	if verbose && changed == 0 && failed == 0 {
		fmt.Fprintf(w, "All profiles are up to date.\n")
	}
	if failed > 0 {
		return fmt.Errorf("%d migration(s) failed", failed)
	}
	return nil
}

// autoMigrate is the announced, non-fatal migration pass the startup hook
// runs once per binary version. Output goes to stderr so piped stdout of
// the actual command stays clean.
func (a *App) autoMigrate(st update.State) {
	if !migrate.AnyPending(a.Store) {
		st.SetMigratedVersion(a.Version)
		return
	}
	fmt.Fprintf(a.Stderr, "claude-profile %s — bringing existing profiles up to date:\n", update.Canonical(a.Version))
	if err := a.applyMigrations(a.Stderr, false); err != nil {
		fmt.Fprintf(a.Stderr, "Warning: %v — run 'claude-profile migrate' to retry.\n", err)
	}
}

// migrationStatus renders the profile × migration matrix.
func (a *App) migrationStatus() error {
	status, err := migrate.Status(a.Store)
	if err != nil {
		return err
	}
	profiles, err := a.Store.List()
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		a.printf("No profiles yet. Create one with: claude-profile create <name>\n")
		return nil
	}
	st := a.Style
	for _, p := range profiles {
		a.printf("%s\n", st.bold(st.cyan(p.Name)))
		for _, mig := range migrate.All {
			state, ok := status[p.Name][mig.ID]
			mark, text := st.yellow("○"), st.yellow("pending")
			if ok {
				mark, text = st.green("●"), string(state)
			}
			a.printf("  %s %s %s %s\n", mark, pad(strconv.Itoa(mig.ID)+" "+mig.Name, st, 16), pad(text, st, 8), st.dim(mig.Summary))
		}
	}
	return nil
}
