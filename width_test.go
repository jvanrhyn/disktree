package main

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayRulesVsWidth(t *testing.T) {
	// Create a styled popup line
	style := lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("15"))
	styledText := style.Render("Hello World")
	
	// Check the difference between rune length and visual width
	runeLen := len([]rune(styledText))
	visualWidth := lipgloss.Width(styledText)
	
	fmt.Printf("Styled text: %q\n", styledText)
	fmt.Printf("Rune length: %d\n", runeLen)
	fmt.Printf("Visual width: %d\n", visualWidth)
	
	if runeLen != visualWidth {
		fmt.Printf("FOUND THE ISSUE: Rune length (%d) != Visual width (%d)\n", runeLen, visualWidth)
	}
}