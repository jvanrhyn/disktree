package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxWidth  int
		expected  string
	}{
		{
			name:     "Simple ASCII - no truncation needed",
			input:    "Hello World",
			maxWidth: 20,
			expected: "Hello World",
		},
		{
			name:     "Simple ASCII - truncation needed",
			input:    "Hello World",
			maxWidth: 5,
			expected: "Hello",
		},
		{
			name:     "Unicode box characters - no truncation",
			input:    "╔══════╗",
			maxWidth: 10,
			expected: "╔══════╗",
		},
		{
			name:     "Unicode box characters - truncation needed",
			input:    "╔══════╗",
			maxWidth: 5,
			expected: "╔════",
		},
		{
			name:     "Mixed content with Unicode",
			input:    "Text ╔══════╗ More",
			maxWidth: 10,
			expected: "Text ╔════",
		},
		{
			name:     "Empty string",
			input:    "",
			maxWidth: 5,
			expected: "",
		},
		{
			name:     "Zero width",
			input:    "Hello",
			maxWidth: 0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToWidth(tt.input, tt.maxWidth)
			
			if result != tt.expected {
				t.Errorf("truncateToWidth(%q, %d) = %q; want %q", 
					tt.input, tt.maxWidth, result, tt.expected)
			}
			
			// Verify that the result doesn't exceed maxWidth
			actualWidth := lipgloss.Width(result)
			if actualWidth > tt.maxWidth {
				t.Errorf("Result width %d exceeds maxWidth %d for input %q", 
					actualWidth, tt.maxWidth, tt.input)
			}
			
			// Verify that the result is valid UTF-8
			if !utf8Valid(result) {
				t.Errorf("Result is not valid UTF-8: %q", result)
			}
		})
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}

func TestOverlayTruncationFix(t *testing.T) {
	// Test that reproduces the original truncation issue
	width, height := 40, 10
	
	// Background that's shorter than the terminal width
	body := strings.Repeat("Short\n", height-1) + "Short"
	
	// Wide popup that would extend beyond terminal width when overlaid
	popup := "╔════════════════════════════════════════════════╗\n" +
		   "║         This is a very wide popup dialog         ║\n" +
		   "╚════════════════════════════════════════════════╝"
	
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	// Find the popup lines
	popupStartLine := -1
	for i, line := range resultLines {
		if strings.Contains(line, "╔") {
			popupStartLine = i
			break
		}
	}
	
	if popupStartLine == -1 {
		t.Fatal("Could not find popup in result")
	}
	
	// Check that the popup lines are properly formatted (not truncated mid-character)
	for i := popupStartLine; i < popupStartLine+3 && i < len(resultLines); i++ {
		line := resultLines[i]
		
		// Verify the line width doesn't exceed terminal width
		actualWidth := lipgloss.Width(line)
		if actualWidth != width {
			t.Errorf("Line %d has incorrect visual width %d, expected %d: %q", 
				i, actualWidth, width, line)
		}
		
		// Verify UTF-8 validity (no broken Unicode characters)
		if !utf8Valid(line) {
			t.Errorf("Line %d contains invalid UTF-8: %q", i, line)
		}
		
		// For lines with box characters, they should still be valid even if truncated
		if strings.ContainsAny(line, "╔╗║╚═") {
			// The line should not end with an invalid UTF-8 sequence
			runes := []rune(line)
			if len(runes) > 0 {
				lastRune := runes[len(runes)-1]
				if lastRune == '\uFFFD' {
					t.Errorf("Line %d ends with replacement character (invalid UTF-8): %q", i, line)
				}
			}
		}
	}
}

func TestDebugWidthIssue(t *testing.T) {
	width, height := 40, 10
	
	// Background that's shorter than the terminal width
	body := strings.Repeat("Short\n", height-1) + "Short"
	
	// Wide popup
	popup := "╔════════════════════════════════════════════════╗\n" +
		   "║         This is a very wide popup dialog         ║\n" +
		   "╚════════════════════════════════════════════════╝"
	
	result := renderOverlay(body, popup, width, height)
	resultLines := strings.Split(result, "\n")
	
	fmt.Printf("Terminal size: %dx%d\n", width, height)
	fmt.Printf("Number of result lines: %d\n", len(resultLines))
	
	for i, line := range resultLines {
		fmt.Printf("Line %d: len=%d, width=%d, content=%q\n", 
			i, len(line), lipgloss.Width(line), line)
	}
}