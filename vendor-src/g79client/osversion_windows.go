//go:build windows

package g79client

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// hostSystemVersion reports the running Windows build in the same form as the
// official interop client. It is only used at runtime.
func hostSystemVersion() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}
