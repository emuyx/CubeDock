//go:build !windows

package core

import "os/exec"

func configureLimboCmd(*exec.Cmd) {}

// attachLimboJob is a no-op outside Windows. The helper also watches stdin,
// which closes when CubeDock exits, so an orphaned helper terminates itself.
func attachLimboJob(*exec.Cmd) (func(), error) { return nil, nil }
