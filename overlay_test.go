package main

import (
	"strings"
	"testing"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderOverlay(t *testing.T) {
	// Test that renderOverlay creates a proper overlay effect
	base := strings.Join([]string{
		"Header line",
		"Content line 1",
		"Content line 2", 
		"Content line 3",
		"Footer line",
	}, "\n")
	
	popup := strings.Join([]string{
		"┌─────────┐",
		"│ POPUP   │",
		"└─────────┘",
	}, "\n")
	
	result := renderOverlay(base, popup, 20, 5)
	lines := strings.Split(result, "\n")
	
	// Should have exactly 5 lines
	if len(lines) != 5 {
		t.Fatalf("Expected 5 lines, got %d", len(lines))
	}
	
	// Check that overlay is properly positioned (popup should be centered)
	// The popup should appear in the middle lines, not replace entire lines with blanks
	if !strings.Contains(result, "POPUP") {
		t.Errorf("Expected popup content to be present in overlay")
	}
	
	// Check that each line has proper display width using lipgloss
	for i, line := range lines {
		displayWidth := lipgloss.Width(line)
		if displayWidth != 20 {
			t.Errorf("Line %d should have display width 20, got %d: %q", i, displayWidth, line)
		}
	}
}

func TestOverlayLineContent(t *testing.T) {
	// Test the overlay line content function
	bgLine := "Background content here"
	popupLine := "POPUP"
	
	result := overlayLineContent(bgLine, popupLine, 5, 25)
	
	// Should have correct display width
	displayWidth := lipgloss.Width(result)
	if displayWidth != 25 {
		t.Errorf("Expected display width 25, got %d: %q", displayWidth, result)
	}
	
	// Should contain the popup content
	if !strings.Contains(result, "POPUP") {
		t.Errorf("Result should contain popup content: %q", result)
	}
}