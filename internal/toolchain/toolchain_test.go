package toolchain

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// melange prints "melange version v0.20.1" or "melange v0.20.1 ..."
		{"melange version v0.20.1", "v0.20.1"},
		{"melange v0.20.1 (commit abc123)", "v0.20.1"},
		// apko prints "apko version v0.27.0"
		{"apko version v0.27.0", "v0.27.0"},
		// grype --version prints "grype 0.80.0"
		{"grype 0.80.0", "v0.80.0"},
		// already normalized
		{"v0.20.1", "v0.20.1"},
		// no match → return as-is
		{"unknown", "unknown"},
	}
	for _, tc := range cases {
		got := normalizeVersion(tc.input)
		if got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
