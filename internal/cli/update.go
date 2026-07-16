package cli

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/victorRadu/claude-profile/internal/update"
)

// httpTimeout bounds every network call the update command makes.
const httpTimeout = 30 * time.Second

func (a *App) updateState() update.State {
	return update.State{Root: a.Store.Root, Now: time.Now}
}

// updateCmd implements `claude-profile update [--check]` and the hidden
// --refresh-cache mode the detached background checker runs in.
func (a *App) updateCmd(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	check := fs.Bool("check", false, "only report whether an update exists (exit 1 if one does)")
	refresh := fs.Bool("refresh-cache", false, "refresh the version cache and exit (used internally)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: claude-profile update [--check]")
	}
	client := &http.Client{Timeout: httpTimeout}

	if *refresh {
		// Background mode: outcome is invisible by design; the cache
		// timestamp advances even on failure (see update.State.Refresh).
		_ = a.updateState().Refresh(client)
		return nil
	}

	if update.IsDevBuild(a.Version) {
		return fmt.Errorf("this is a development build (%s) — rebuild from your checkout (make build) or use the installer", a.Version)
	}

	rel, err := update.LatestRelease(client)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	st := a.Style
	if !update.Newer(rel.Version, a.Version) {
		a.printf("%s claude-profile %s is up to date %s\n",
			st.green("✓"), st.bold(update.Canonical(a.Version)), st.dim("(latest: "+rel.Version+")"))
		if *check {
			return nil
		}
		// The binary is current, but profiles may still predate features.
		return a.applyMigrations(a.Stdout, true)
	}
	if *check {
		a.printf("↑ claude-profile %s is available (you have %s) — run: claude-profile update\n",
			rel.Version, update.Canonical(a.Version))
		return errSilentExit
	}

	a.printf("Updating claude-profile %s → %s\n", update.Canonical(a.Version), st.bold(rel.Version))
	binary, err := update.Download(client, rel)
	if err != nil {
		return err
	}
	a.printf("%s Downloaded %s %s\n", st.green("✓"), update.AssetName(rel.Version), st.dim("(checksum verified)"))
	if err := update.Apply(a.Exe, binary); err != nil {
		return err
	}
	a.printf("%s Installed to %s\n", st.green("✓"), displayPath(a.Exe))

	// Migrations must run the NEW binary's code, so hand over to it.
	cmd := exec.Command(a.Exe, "migrate")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, a.Stdout, a.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(a.Stderr, "Warning: the update succeeded, but migrations failed (%v) — run 'claude-profile migrate' to retry.\n", err)
	}
	a.printf("\nDone — claude-profile %s.\n", st.bold(rel.Version))
	return nil
}

// startupHook runs before normal commands: it cleans update leftovers,
// auto-applies pending profile migrations once per binary version, prints
// the cached update notice, and keeps the version cache fresh — all without
// ever blocking on the network. Machine-facing and self-referential
// commands are exempt, as are dev builds.
func (a *App) startupHook(cmd string) {
	if hookExempt[cmd] {
		return
	}
	update.CleanupOld(a.Exe)
	if update.IsDevBuild(a.Version) || !a.Interactive {
		return
	}
	st := a.updateState()
	if st.MigratedVersion() != a.Version {
		a.autoMigrate(st)
	}
	if !update.Enabled(a.Version) {
		return
	}
	if msg := st.Notice(a.Version); msg != "" {
		fmt.Fprintln(a.Stderr, a.Style.dim(msg))
	}
	if st.Stale() {
		update.SpawnRefresh(a.Exe)
	}
}

// hookExempt lists commands the startup hook must stay away from:
// machine-facing paths where extra output or work is wrong, and the
// update/migrate machinery itself.
var hookExempt = map[string]bool{
	"statusline": true, "wrap-exec": true,
	"update": true, "migrate": true,
	"version": true, "-v": true, "--version": true,
	"help": true, "-h": true, "--help": true,
}
