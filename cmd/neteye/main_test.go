package main

import (
	"strings"
	"testing"
)

func TestGetUsageTemplate(t *testing.T) {
	// Test with colors disabled
	globalNoColor = true
	template := getUsageTemplate()

	if !strings.Contains(template, "███╗   ██╗███████╗████████╗███████╗██╗   ██╗███████╗") {
		t.Error("Template should contain the logo")
	}

	// Should not contain color codes when colors are disabled
	if strings.Contains(template, "\033[36m") {
		t.Error("Template should not contain color codes when colors are disabled")
	}

	// Test with colors enabled
	globalNoColor = false
	template = getUsageTemplate()

	if !strings.Contains(template, "███╗   ██╗███████╗████████╗███████╗██╗   ██╗███████╗") {
		t.Error("Template should contain the logo")
	}

	// Should contain color codes when colors are enabled
	if !strings.Contains(template, "\033[36m") {
		t.Error("Template should contain color codes when colors are enabled")
	}

	// Reset to default
	globalNoColor = false
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"999", 999, false},
		{"abc", 0, true},
		{"12a", 0, true},
		{"", 0, true},
		{"-123", 0, true}, // negative numbers should error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInt(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input %q, but got: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d for input %q, got %d", tt.expected, tt.input, result)
				}
			}
		})
	}
}

func TestGlobalVariables(t *testing.T) {
	// Test default values
	if globalFormat != "table" {
		t.Errorf("Expected default format to be 'table', got %q", globalFormat)
	}

	if globalSortBy != "proto" {
		t.Errorf("Expected default sort to be 'proto', got %q", globalSortBy)
	}

	if globalNoColor != false {
		t.Errorf("Expected default no-color to be false, got %v", globalNoColor)
	}

	if globalResolve != false {
		t.Errorf("Expected default resolve to be false, got %v", globalResolve)
	}
}

func TestVersion(t *testing.T) {
	if version == "" {
		t.Error("Version should not be empty")
	}

	if !strings.HasPrefix(version, "v") {
		t.Errorf("Version should start with 'v', got %q", version)
	}
}

func TestLogo(t *testing.T) {
	if logo == "" {
		t.Error("Logo should not be empty")
	}

	// Logo should contain the expected ASCII art
	expectedLines := []string{
		"███╗   ██╗███████╗████████╗███████╗██╗   ██╗███████╗",
		"████╗  ██║██╔════╝╚══██╔══╝██╔════╝╚██╗ ██╔╝██╔════╝",
		"██╔██╗ ██║█████╗     ██║   █████╗   ╚████╔╝ █████╗",
		"██║╚██╗██║██╔══╝     ██║   ██╔══╝    ╚██╔╝  ██╔══╝",
		"██║ ╚████║███████╗   ██║   ███████╗   ██║   ███████╗",
		"╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚══════╝   ╚═╝   ╚══════╝",
	}

	for _, line := range expectedLines {
		if !strings.Contains(logo, line) {
			t.Errorf("Logo should contain line: %q", line)
		}
	}
}
