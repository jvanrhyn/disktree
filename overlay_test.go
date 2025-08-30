package main

import (
	"strings"
	"testing"
	"github.com/charmbracelet/lipgloss"
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

func TestDebugOverlayLogic(t *testing.T) {
	// Debug the overlay logic step by step
	base := "📁 Music                                                     32.2 MB     143     14          0.1%        ░░░"
	popup := "┌──────────────────────────────────┐\n│        Scanning files...         │\n└──────────────────────────────────┘"
	
	width := 120
	
	t.Logf("Base line: %q", base)
	t.Logf("Base line width: %d", lipgloss.Width(base))
	
	popLines := strings.Split(popup, "\n")
	t.Logf("Popup lines: %v", popLines)
	
	for i, popLine := range popLines {
		t.Logf("Popup line %d: %q (width: %d)", i, popLine, lipgloss.Width(popLine))
	}
	
	// Test the middle popup line (index 1)
	popupLine := popLines[1] // "│        Scanning files...         │"
	popupWidth := lipgloss.Width(popupLine)
	
	// Calculate popup position (centered)
	startCol := (width - popupWidth) / 2
	
	t.Logf("Popup width: %d, start column: %d", popupWidth, startCol)
	
	// Test the helper functions
	beforePopup := truncateToWidth(base, startCol)
	t.Logf("Before popup (truncate to %d): %q", startCol, beforePopup)
	
	popupEndCol := startCol + popupWidth
	afterPopup := extractAfterPosition(base, popupEndCol)
	t.Logf("After popup (extract from %d): %q", popupEndCol, afterPopup)
	
	result := beforePopup + popupLine + afterPopup
	t.Logf("Combined result: %q", result)
	t.Logf("Combined result width: %d", lipgloss.Width(result))
}

func TestOverlayPreservesContentAfterPopup(t *testing.T) {
	// Test the actual use case: a full table view with popup overlay
	header := "DiskTree TUI — /home/user"
	tableRows := []string{
		"📁 Music                                                     32.2 MB     143     14          0.1%        ░░░",
		"📁 .anydesk                                                  32.2 MB     20      9           0.1%        ░░░",
		"📁 .BurpSuite                                                31.0 MB     1845    149         0.1%        ░░░",
		"📁 temp                                                      16.4 MB     2375    187         0.0%        ░░░",
		"📁 .dbclient                                                 15.9 MB     2       2           0.0%        ░░░",
	}
	status := "Status line"
	footer := "↑/↓ move  Enter open  q=quit"
	
	// Construct the full body as the actual app would
	allLines := append([]string{header}, tableRows...)
	allLines = append(allLines, status, footer)
	body := strings.Join(allLines, "\n")
	
	popup := "┌──────────────────────────────────┐\n│        Scanning files...         │\n└──────────────────────────────────┘"
	
	width := 120
	height := len(allLines)
	
	result := renderOverlay(body, popup, width, height)
	lines := strings.Split(result, "\n")
	
	// Debug all lines
	for i, line := range lines {
		t.Logf("Line %d: %q", i, line)
	}
	
	// Find lines that should have popup overlay (they should be in the middle of the screen)
	// The popup has 3 lines and should be centered vertically
	popupStartRow := (height - 3) / 2
	
	// Check the middle popup line
	if popupStartRow+1 < len(lines) {
		overlayLine := lines[popupStartRow+1]
		
		// Should contain original table content before popup
		if !strings.Contains(overlayLine, "📁") {
			t.Errorf("Overlay line missing file icon. Line: %q", overlayLine)
		}
		
		// Should contain the popup box content
		if !strings.Contains(overlayLine, "Scanning files") {
			t.Errorf("Overlay line missing popup content. Line: %q", overlayLine)  
		}
		
		// Should contain content after popup (file size, counts, etc.)
		if !strings.Contains(overlayLine, "149") || !strings.Contains(overlayLine, "░") {
			t.Errorf("Overlay line missing content after popup. Line: %q", overlayLine)
		}
	}
}