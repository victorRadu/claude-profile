package update

import (
	"os/exec"
)

// SpawnRefresh starts exe detached to refresh the version cache in the
// background ("update --refresh-cache"). The foreground never waits and
// never touches the network; failures here are invisible by design.
func SpawnRefresh(exe string) {
	cmd := exec.Command(exe, "update", "--refresh-cache")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detachAttrs(cmd)
	if err := cmd.Start(); err != nil {
		return
	}
	_ = cmd.Process.Release()
}
