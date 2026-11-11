package netease

import "testing"

func TestDynamicTokenVectors(t *testing.T) {
	for _, vector := range []struct {
		token, path, body, gateway, core string
	}{
		{"fixture-token", "/user-addme-friends/", "{}", "NClpuGGtdPsp666u1", "CKjURyMQiwSo1yoQ1"},
		{"other-fixture", "/user-reply-friend/", `{"fid":123456789,"accept":true,"accept_ui":"联系人-好友申请"}`, "u3hsKb7rLzY1uijv1", "h4evVwMoE0iIByjT1"},
	} {
		if got := DynamicToken(vector.token, vector.path, []byte(vector.body), false); got != vector.gateway {
			t.Fatalf("gateway signature for %s = %q, want %q", vector.token, got, vector.gateway)
		}
		if got := DynamicToken(vector.token, vector.path, []byte(vector.body), true); got != vector.core {
			t.Fatalf("core signature for %s = %q, want %q", vector.token, got, vector.core)
		}
	}
}
