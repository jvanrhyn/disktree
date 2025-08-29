package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayBorderAlignment(t *testing.T) {
	// Test with a simple border to see if alignment is correct
	width, height := 80, 24
	
	// Create background
	body := strings.Repeat("Background Content Line\n", height-1) + "Background Content Line"
	
	// Create a popup with border
	popupW := 20
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Width(popupW)
	
	popup := modalStyle.Render("Test Content")
	
	// Render the overlay
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	// Find the popup in the result
	popupTopLine := -1
	for i, line := range resultLines {
		if strings.Contains(line, "╔") { // Look for top border
			popupTopLine = i
			break
		}
	}
	
	if popupTopLine == -1 {
		t.Fatal("Could not find popup border in result")
	}
	
	// Calculate expected position
	popupLines := strings.Split(popup, "\n")
	popW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popW {
			popW = w
		}
	}
	
	expectedStartRow := (height - len(popupLines)) / 2
	expectedStartCol := (width - popW) / 2
	
	// Check row alignment
	if popupTopLine != expectedStartRow {
		t.Errorf("Popup row not centered. Expected row %d, found at row %d", expectedStartRow, popupTopLine)
	}
	
	// Check column alignment by finding where the border starts
	borderLine := resultLines[popupTopLine]
	borderStart := strings.Index(borderLine, "╔")
	
	if borderStart != expectedStartCol {
		t.Errorf("Popup column not centered. Expected column %d, found at column %d", expectedStartCol, borderStart)
		t.Logf("Border line: %q", borderLine)
		t.Logf("Expected popup width: %d", popW)
		t.Logf("Expected start column calculation: (%d-%d)/2 = %d", width, popW, expectedStartCol)
	}
}