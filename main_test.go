package main

import (
	"strings"
	"testing"
)

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500 B"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1099511627776, "1.0 TB"},
	}
	for _, c := range cases {
		got := humanBytes(c.in)
		if got != c.want {
			t.Fatalf("humanBytes(%d) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestBar(t *testing.T) {
	// width 10, p=0 => all empty
	if got := bar(0, 10); got != strings.Repeat("░", 10) {
		t.Fatalf("bar(0,10) = %q; want %q", got, strings.Repeat("░", 10))
	}

	// width 10, p=1 => all filled
	if got := bar(1, 10); got != strings.Repeat("█", 10) {
		t.Fatalf("bar(1,10) = %q; want %q", got, strings.Repeat("█", 10))
	}

	// half filled
	if got := bar(0.5, 10); got != strings.Repeat("█", 5)+strings.Repeat("░", 5) {
		t.Fatalf("bar(0.5,10) = %q; want %q", got, strings.Repeat("█", 5)+strings.Repeat("░", 5))
	}

	// width <= 0 should default to 10
	if got := bar(0.5, 0); got != strings.Repeat("█", 5)+strings.Repeat("░", 5) {
		t.Fatalf("bar(0.5,0) = %q; want %q", got, strings.Repeat("█", 5)+strings.Repeat("░", 5))
	}

	// p > 1 should clamp to full width
	if got := bar(2, 10); got != strings.Repeat("█", 10) {
		t.Fatalf("bar(2,10) = %q; want %q", got, strings.Repeat("█", 10))
	}
}

func TestMax(t *testing.T) {
	if got := maxvalue(1, 2); got != 2 {
		t.Fatalf("max(1,2) = %d; want 2", got)
	}
	if got := maxvalue(5, -1); got != 5 {
		t.Fatalf("max(5,-1) = %d; want 5", got)
	}
}
