package minecraft

import "testing"

func TestValidResourcePackChunk(t *testing.T) {
	cases := []struct {
		name  string
		size  uint64
		chunk uint32
		ok    bool
	}{
		{"normal", 1024, 128 << 10, true},
		{"zero size", 0, 128 << 10, true},
		{"max size", maxResourcePackSize, 128 << 10, true},
		{"oversized", maxResourcePackSize + 1, 128 << 10, false},
		{"overflow", 1 << 63, 128 << 10, false},
		{"zero chunk", 1024, 0, false},
		{"max chunk", 1024, maxResourcePackChunkSize, true},
		{"oversized chunk", 1024, maxResourcePackChunkSize + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validResourcePackChunk(tc.size, tc.chunk); got != tc.ok {
				t.Fatalf("validResourcePackChunk(%d, %d) = %v, want %v", tc.size, tc.chunk, got, tc.ok)
			}
		})
	}
}
