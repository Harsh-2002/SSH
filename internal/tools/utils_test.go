package tools

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"", "''"},
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"$(command)", "'$(command)'"},
		{"; rm -rf /", "'; rm -rf /'"},
		{"`whoami`", "'`whoami`'"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := shellQuote(tc.input)
			if result != tc.expected {
				t.Errorf("shellQuote(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	testCases := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"", "", true},
		{"abc", "", true},
		{"", "abc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.s+"_"+tc.substr, func(t *testing.T) {
			result := containsString(tc.s, tc.substr)
			if result != tc.expected {
				t.Errorf("containsString(%q, %q) = %v, want %v", tc.s, tc.substr, result, tc.expected)
			}
		})
	}
}

func TestTrimOutput(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\n\ntest\n\n", "test"},
		{"\t  spaces \t", "spaces"},
		{"no trim", "no trim"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := trimOutput(tc.input)
			if result != tc.expected {
				t.Errorf("trimOutput(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
