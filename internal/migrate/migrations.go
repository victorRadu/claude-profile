package migrate

import (
	"github.com/victorRadu/claude-profile/internal/profile"
	"github.com/victorRadu/claude-profile/internal/statusline"
)

// All is the append-only migration registry, in ID order.
//
// Rules for adding an entry (see docs/updates.md):
//   - take the next ID; never renumber, reorder, or delete entries
//   - Apply must be idempotent and skip cleanly when already present
//   - make `create` produce the same end state for new profiles
//   - never touch credentials, history, or foreign configuration
var All = []Migration{
	{
		ID:      1,
		Name:    "statusline",
		Summary: "status line shows profile and model",
		Apply:   applyStatusline,
	},
}

// applyStatusline gives pre-statusline profiles the status line that create
// now installs. An existing foreign status line is chained, never replaced
// (statusline.Install stashes it); an entry that is already ours is a skip.
func applyStatusline(p profile.Profile, ctx Context) (Result, string, error) {
	installed, err := statusline.Installed(p.Dir)
	if err != nil {
		return Skipped, "", err
	}
	if installed {
		return Skipped, "already set up", nil
	}
	chained, err := statusline.Install(p.Dir, ctx.Exe)
	if err != nil {
		return Skipped, "", err
	}
	if chained {
		return Applied, "existing status line kept, shown after it", nil
	}
	return Applied, "", nil
}
