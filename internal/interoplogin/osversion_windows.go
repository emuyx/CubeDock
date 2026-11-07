//go:build windows

package interoplogin

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// hostSystemVersion reports the running Windows build in the same form as the
// official interop client. The value is only used at runtime; it is not written
// to the repository.
func hostSystemVersion() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}
