package core

import "testing"

func TestManagedPackName(t *testing.T) {
	valid := []string{
		"b41c2785-c512-4a49-af56-3a87afd47c57_1.21.30.mcpack",
		"00000000-0000-0000-0000-000000000000_0.0.1.mcpack",
	}
	for _, name := range valid {
		if !managedPackName.MatchString(name) {
			t.Fatalf("%q should be managed", name)
		}
	}
	invalid := []string{
		"custom-pack.mcpack",
		"b41c2785-c512-4a49-af56-3a87afd47c57_1.21.30.zip",
		"b41c2785-c512-4a49-af56-3a87afd47c57_1.21.30.extra.mcpack",
		"not-a-uuid_1.21.30.mcpack",
	}
	for _, name := range invalid {
		if managedPackName.MatchString(name) {
			t.Fatalf("%q should not be managed", name)
		}
	}
}
