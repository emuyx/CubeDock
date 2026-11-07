//go:build !windows

package interoplogin

// hostSystemVersion is unavailable outside Windows; systemVersion falls back to
// the generic Windows 11 build value.
func hostSystemVersion() string { return "" }
