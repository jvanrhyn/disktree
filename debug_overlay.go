package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func debugOverlay() {
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
}