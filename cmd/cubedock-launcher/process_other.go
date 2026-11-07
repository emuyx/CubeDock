//go:build !windows

package main

import "os/exec"

func configureLauncherChild(*exec.Cmd) {}

func attachLauncherJob(*exec.Cmd) (func(), error) { return nil, nil }
