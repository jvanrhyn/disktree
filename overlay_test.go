package main

import (
	"strings"
	"testing"
)

func TestRenderOverlay(t *testing.T) {
	// Test basic overlay functionality
	base := "Hello World\nSecond Line\nThird Line"
	popup := "POPUP"
	width := 11
	height := 3

	result := renderOverlay(base, popup, width, height)
	lines := strings.Split(result, "\n")

	if len(lines) != height {
		t.Fatalf("Expected %d lines, got %d", height, len(lines))
	}

	// Check that each line has the correct width
	for i, line := range lines {
		if len(line) != width {
			t.Fatalf("Line %d has width %d, expected %d: %q", i, len(line), width, line)
		}
	}

	// The popup should be centered on line 1 (middle line)
	// Expected behavior: popup should overlay on the background, preserving visible parts
	expectedMiddleLine := "SecPOPUPine" // "Second Line" with "POPUP" overlaid in the center
	actualMiddleLine := lines[1]

	if actualMiddleLine != expectedMiddleLine {
		t.Fatalf("Overlay not working correctly.\nExpected: %q\nActual:   %q", expectedMiddleLine, actualMiddleLine)
	}

	// First and last lines should remain unchanged (except for padding)
	expectedFirstLine := "Hello World"
	expectedThirdLine := "Third Line "
	
	if lines[0] != expectedFirstLine {
		t.Fatalf("First line changed unexpectedly.\nExpected: %q\nActual:   %q", expectedFirstLine, lines[0])
	}
	
	if lines[2] != expectedThirdLine {
		t.Fatalf("Third line changed unexpectedly.\nExpected: %q\nActual:   %q", expectedThirdLine, lines[2])
	}
}

func TestRenderOverlayPreservesBackground(t *testing.T) {
	// Test that background content is preserved when overlaying popup
	base := "ABCDEFGHIJKLMNOP"  // 16 characters
	popup := "XYZ"              // 3 characters
	width := 16
	height := 1

	result := renderOverlay(base, popup, width, height)
	lines := strings.Split(result, "\n")

	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	line := lines[0]
	if len(line) != width {
		t.Fatalf("Line has width %d, expected %d", len(line), width)
	}

	// With a 3-character popup centered in a 16-character width:
	// startCol = (16-3)/2 = 6
	// Expected result: "ABCDEFXYZJKLMNOP"
	// The popup "XYZ" should replace characters at positions 6, 7, 8
	expected := "ABCDEFXYZJKLMNOP"
	
	if line != expected {
		t.Fatalf("Overlay does not preserve background correctly.\nExpected: %q\nActual:   %q", expected, line)
	}
}

func TestRenderOverlayEdgeCases(t *testing.T) {
	// Test empty popup
	base := "Hello World"
	popup := ""
	width := 11
	height := 1

	result := renderOverlay(base, popup, width, height)
	lines := strings.Split(result, "\n")
	
	if lines[0] != base {
		t.Fatalf("Empty popup should not change background. Expected: %q, Got: %q", base, lines[0])
	}

	// Test popup larger than background
	base = "Hi"
	popup = "Very Long Popup Text"
	width = 20
	height = 1

	result = renderOverlay(base, popup, width, height)
	lines = strings.Split(result, "\n")
	
	// Should overlay as much as possible
	if len(lines[0]) != width {
		t.Fatalf("Result line should have correct width %d, got %d", width, len(lines[0]))
	}

	// Test multi-line popup
	base = "Line1\nLine2\nLine3"
	popup = "POP1\nPOP2"
	width = 6
	height = 3

	result = renderOverlay(base, popup, width, height)
	lines = strings.Split(result, "\n")
	
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lines))
	}
	
	// First two lines should have popup overlaid, third should be unchanged
	expectedLine1 := "LPOP1 " // "Line1 " with "POP1" overlaid at center (pos 1-4)
	expectedLine2 := "LPOP2 " // "Line2 " with "POP2" overlaid at center (pos 1-4)
	expectedLine3 := "Line3 " // "Line3" unchanged but padded
	
	if lines[0] != expectedLine1 {
		t.Fatalf("Line 0 incorrect. Expected: %q, Got: %q", expectedLine1, lines[0])
	}
	if lines[1] != expectedLine2 {
		t.Fatalf("Line 1 incorrect. Expected: %q, Got: %q", expectedLine2, lines[1])
	}
	if lines[2] != expectedLine3 {
		t.Fatalf("Line 2 incorrect. Expected: %q, Got: %q", expectedLine3, lines[2])
	}
}