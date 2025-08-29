package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRealWorldOverlayAlignment(t *testing.T) {
	// Exactly match the real application usage
	width, height := 80, 24
	
	// Create background similar to the real TUI
	head := "DiskTree TUI — /some/path"
	status := "Select item to delete"
	foot := "↑/↓ move  Enter open  Backspace up  s=size  n=name  r=rescan  e=export CSV  d=delete  u=undo  q=quit"
	
	body := lipgloss.JoinVertical(lipgloss.Left,
		head,
		"File listing would go here...",
		strings.Repeat("Some file entry\n", height-5),
		status,
		foot,
	)
	
	// Create popup exactly like in the real app
	popupW := 60
	popupW = minvalue(popupW, maxvalue(10, width-4))
	
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Padding(1, 2).
		Width(popupW).
		Align(lipgloss.Center).
		Background(lipgloss.Color("0"))
		
	// Buttons exactly like in real app
	btnYes := lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
	btnNo := lipgloss.NewStyle().Padding(0, 2)
	
	yes := btnYes.Render(" Yes ")
	no := btnNo.Render(" No ")
	content := lipgloss.JoinHorizontal(lipgloss.Center, "Do you really want to delete 'some-file.txt'?")
	footer := lipgloss.JoinHorizontal(lipgloss.Center, yes, " ", no)
	
	popup := modalStyle.Render(lipgloss.JoinVertical(lipgloss.Center, content, "", footer))
	
	// Analyze the popup
	popupLines := strings.Split(popup, "\n")
	fmt.Printf("Real popup analysis:\n")
	fmt.Printf("Popup has %d lines\n", len(popupLines))
	
	popW := 0
	for i, l := range popupLines {
		w := lipgloss.Width(l)
		if w > popW {
			popW = w
		}
		fmt.Printf("Line %d: width=%d, content=%q\n", i, w, l)
	}
	
	fmt.Printf("Calculated popup width: %d\n", popW)
	
	// Calculate expected centering
	expectedStartRow := (height - len(popupLines)) / 2
	expectedStartCol := (width - popW) / 2
	
	fmt.Printf("Expected position: row=%d, col=%d\n", expectedStartRow, expectedStartCol)
	
	// Render overlay
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	// Find where popup actually appears
	actualStartRow := -1
	actualStartCol := -1
	
	for i, line := range resultLines {
		if strings.Contains(line, "╔") {
			actualStartRow = i
			actualStartCol = strings.Index(line, "╔")
			break
		}
	}
	
	if actualStartRow == -1 {
		t.Fatal("Could not find popup in result")
	}
	
	fmt.Printf("Actual position: row=%d, col=%d\n", actualStartRow, actualStartCol)
	
	// Check for alignment issues
	if actualStartRow != expectedStartRow {
		t.Errorf("Row alignment off by %d. Expected %d, got %d", actualStartRow-expectedStartRow, expectedStartRow, actualStartRow)
	}
	
	if actualStartCol != expectedStartCol {
		t.Errorf("Column alignment off by %d. Expected %d, got %d", actualStartCol-expectedStartCol, expectedStartCol, actualStartCol)
	}
	
	// Print the lines around the popup for visual inspection
	fmt.Printf("\nResult around popup:\n")
	for i := maxvalue(0, actualStartRow-2); i <= minvalue(len(resultLines)-1, actualStartRow+len(popupLines)+1); i++ {
		fmt.Printf("Line %2d: %q\n", i, resultLines[i])
	}
}