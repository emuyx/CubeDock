//go:build windows

package netease

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func hostSystemVersion() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}
