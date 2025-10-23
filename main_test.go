package main

import (
	"testing"
)

func TestMimeTypeFromExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"output.png", "image/png"},
		{"output.jpg", "image/jpeg"},
		{"output.jpeg", "image/jpeg"},
		{"output.PNG", "image/png"},
		{"output.JPG", "image/jpeg"},
		{"output", "image/png"},      // No extension defaults to PNG
		{"output.txt", "image/png"},  // Unknown extension defaults to PNG
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := mimeTypeFromExtension(tt.filename)
			if result != tt.expected {
				t.Errorf("mimeTypeFromExtension(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}
