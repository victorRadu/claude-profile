// Command claude-profile manages isolated Claude Code profiles.
//
// Each profile is a separate configuration directory (CLAUDE_CONFIG_DIR),
// giving it its own login, settings, skills, agents and history.
package main

import (
	"os"

	"github.com/victorRadu/claude-profile/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
