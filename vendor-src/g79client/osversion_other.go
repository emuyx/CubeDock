//go:build !windows

package g79client

// hostSystemVersion is unavailable outside Windows; interopOSVersion falls back
// to the generic Windows 11 build value.
func hostSystemVersion() string { return "" }
