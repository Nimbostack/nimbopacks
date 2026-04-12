package main

import (
	"io"
	"strings"
	"testing"
)

func TestPromptOverwrite_Yes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n", "Yes\n"} {
		r := strings.NewReader(input)
		got, err := promptOverwrite("/some/dir/nimpack.yaml", r, io.Discard)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if !got {
			t.Errorf("input %q: want true, got false", input)
		}
	}
}

func TestPromptOverwrite_No(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "no\n", "\n", "anything\n"} {
		r := strings.NewReader(input)
		got, err := promptOverwrite("/some/dir/nimpack.yaml", r, io.Discard)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if got {
			t.Errorf("input %q: want false, got true", input)
		}
	}
}

func TestPromptOverwrite_EOF(t *testing.T) {
	r := strings.NewReader("") // EOF immediately — non-interactive pipe
	got, err := promptOverwrite("/some/dir/nimpack.yaml", r, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
	if got {
		t.Error("want false on EOF, got true")
	}
}

func TestPromptConfirm_Yes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n", "Yes\n"} {
		r := strings.NewReader(input)
		got, err := promptConfirm("confirm? [y/N]: ", r, io.Discard)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if !got {
			t.Errorf("input %q: want true, got false", input)
		}
	}
}

func TestPromptConfirm_No(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "no\n", "\n", "anything\n"} {
		r := strings.NewReader(input)
		got, err := promptConfirm("confirm? [y/N]: ", r, io.Discard)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if got {
			t.Errorf("input %q: want false, got true", input)
		}
	}
}

func TestPromptConfirm_EOF(t *testing.T) {
	r := strings.NewReader("")
	got, err := promptConfirm("confirm? [y/N]: ", r, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
	if got {
		t.Error("want false on EOF, got true")
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1395864371, "1.3 GB"},
	}
	for _, c := range cases {
		got := formatBytes(c.in)
		if got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
