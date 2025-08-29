package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDetailedOverlayTrace(t *testing.T) {
	width, height := 80, 10 // Smaller for easier debugging
	
	// Simple background 
	body := strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ\n", height-1) + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	
	// Simple popup
	popup := "╔══════╗\n║ TEST ║\n╚══════╝"
	
	// First let's see what renderOverlay calculates
	popupLines := strings.Split(popup, "\n")
	fmt.Printf("Popup lines:\n")
	for i, line := range popupLines {
		fmt.Printf("  Line %d: width=%d, runes=%d, content=%q\n", i, lipgloss.Width(line), len([]rune(line)), line)
	}
	
	popW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popW {
			popW = w
		}
	}
	popH := len(popupLines)
	
	startRow := (height - popH) / 2
	startCol := (width - popW) / 2
	
	fmt.Printf("Calculated: popW=%d, popH=%d, startRow=%d, startCol=%d\n", popW, popH, startRow, startCol)
	
	// Now render and see what happens
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	fmt.Printf("Result lines:\n")
	for i, line := range resultLines {
		fmt.Printf("  Line %d: %q\n", i, line)
	}
	
	// Check specific overlay lines
	for i := startRow; i < startRow+popH && i < len(resultLines); i++ {
		line := resultLines[i]
		if !strings.Contains(line, "╔") && !strings.Contains(line, "║") && !strings.Contains(line, "╚") {
			t.Errorf("Line %d should contain popup content but doesn't: %q", i, line)
		}
	}
}