package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDebugOverlayAlignment(t *testing.T) {
	// Simulate the exact scenario from the View function
	width, height := 80, 24
	
	// Create a simple background body similar to the TUI
	body := strings.Repeat("Background Content Line\n", height-1) + "Background Content Line"
	
	// Create a popup with border and styling like in the actual app
	popupW := 40
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Padding(1, 2).
		Width(popupW).
		Align(lipgloss.Center).
		Background(lipgloss.Color("0"))
	
	content := "Do you really want to delete this item?"
	footer := " Yes   No "
	popup := modalStyle.Render(lipgloss.JoinVertical(lipgloss.Center, content, "", footer))
	
	// Print popup dimensions for debugging
	popupLines := strings.Split(popup, "\n")
	fmt.Printf("Popup has %d lines\n", len(popupLines))
	for i, line := range popupLines {
		fmt.Printf("Line %d: width=%d content=%q\n", i, lipgloss.Width(line), line)
	}
	
	// Check what renderOverlay thinks the popup width is
	popW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popW {
			popW = w
		}
	}
	fmt.Printf("Calculated popup width: %d\n", popW)
	fmt.Printf("Expected start column: %d\n", (width-popW)/2)
	
	// Render the overlay
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	// Show the middle lines where the popup should be
	startRow := (height - len(popupLines)) / 2
	fmt.Printf("Expected start row: %d\n", startRow)
	fmt.Printf("Popup should appear on lines %d to %d\n", startRow, startRow+len(popupLines)-1)
	
	// Print a few lines around where the popup should be
	for i := startRow - 2; i <= startRow + len(popupLines) + 1 && i < len(resultLines); i++ {
		if i >= 0 {
			fmt.Printf("Result line %d: %q\n", i, resultLines[i])
		}
	}
	
	// Check if the popup appears to be correctly centered
	if len(popupLines) > 0 {
		// Find the first popup line in the result
		popupStartRow := -1
		for i, line := range resultLines {
			if strings.Contains(line, "Do you really want to delete") {
				popupStartRow = i
				break
			}
		}
		
		if popupStartRow == -1 {
			t.Fatal("Could not find popup content in result")
		}
		
		expectedStartRow := (height - len(popupLines)) / 2
		if popupStartRow != expectedStartRow {
			t.Errorf("Popup row not centered. Expected row %d, found at row %d", expectedStartRow, popupStartRow)
		}
		
		// Check column alignment by looking at the popup line
		popupLine := resultLines[popupStartRow]
		
		// Find where the popup content starts in the line
		contentStart := strings.Index(popupLine, "Do you really want to delete")
		if contentStart == -1 {
			t.Fatal("Could not find popup content in the line")
		}
		
		expectedStartCol := (width - popW) / 2
		
		// The content might not start exactly at startCol due to border/padding,
		// but it should be close
		if abs(contentStart - expectedStartCol) > 5 { // Allow some tolerance for borders/padding
			t.Errorf("Popup column not centered. Expected around column %d, found at column %d", expectedStartCol, contentStart)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}