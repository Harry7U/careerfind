package main

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestEmailRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid_simple", "test@example.com", true},
		{"valid_complex", "user.name+tag@example.co.uk", true},
		{"invalid_no_domain", "user@", false},
		{"invalid_no_at", "userexample.com", false},
		{"invalid_multiple_at", "user@domain@example.com", false},
		{"valid_numbers", "user123@example456.com", true},
		{"valid_dots", "first.last@example.com", true},
		{"invalid_special_chars", "user#@example.com", false},
	}

	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := emailRegex.MatchString(tt.input)
			if got != tt.expected {
				t.Errorf("emailRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResult_Validation(t *testing.T) {
	tests := []struct {
		name    string
		result  Result
		wantErr bool
	}{
		{
			name: "valid_result",
			result: Result{
				Emails:    []string{"test@example.com"},
				Location:
