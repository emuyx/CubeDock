//go:build !windows

package console

// EnableUTF8 is a no-op outside Windows; terminals already use UTF-8.
func EnableUTF8() {}
