package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestGetenv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		def      string
		expected string
	}{
		{
			name:     "environment variable exists",
			key:      "TEST_VAR",
			value:    "test_value",
			def:      "default",
			expected: "test_value",
		},
		{
			name:     "environment variable doesn't exist returns default",
			key:      "NONEXISTENT_VAR",
			value:    "",
			def:      "default",
			expected: "default",
		},
		{
			name:     "environment variable empty returns default",
			key:      "EMPTY_VAR",
			value:    "",
			def:      "default",
			expected: "default",
		},
		{
			name:     "default is empty string",
			key:      "NONEXISTENT_VAR",
			value:    "",
			def:      "",
			expected: "",
		},
		{
			name:     "environment variable with spaces",
			key:      "VAR_WITH_SPACES",
			value:    "value with spaces",
			def:      "default",
			expected: "value with spaces",
		},
		{
			name:     "environment variable with special characters",
			key:      "VAR_SPECIAL",
			value:    "value@#$%",
			def:      "default",
			expected: "value@#$%",
		},
		{
			name:     "environment variable overrides default",
			key:      "OVERRIDE_VAR",
			value:    "override",
			def:      "default",
			expected: "override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			oldValue := os.Getenv(tt.key)
			defer func() {
				if oldValue != "" {
					os.Setenv(tt.key, oldValue)
				} else {
					os.Unsetenv(tt.key)
				}
			}()

			// Set up test environment
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getenv(tt.key, tt.def)
			if result != tt.expected {
				t.Errorf("getenv(%q, %q) = %q, want %q", tt.key, tt.def, result, tt.expected)
			}
		})
	}
}

func TestNewPoolFromEnv_Validation(t *testing.T) {
	// Save original env vars
	originalEnv := map[string]string{
		"PGHOST":     os.Getenv("PGHOST"),
		"PGUSER":     os.Getenv("PGUSER"),
		"PGPASSWORD": os.Getenv("PGPASSWORD"),
		"PGDATABASE": os.Getenv("PGDATABASE"),
		"PGPORT":     os.Getenv("PGPORT"),
		"PGSSLMODE":  os.Getenv("PGSSLMODE"),
	}

	// Restore original env vars after test
	defer func() {
		for k, v := range originalEnv {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing PGHOST",
			envVars: map[string]string{
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "missing PGUSER",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "missing PGPASSWORD",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "missing PGDATABASE",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name:        "missing all required vars",
			envVars:     map[string]string{},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "missing multiple required vars",
			envVars: map[string]string{
				"PGHOST": "localhost",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "empty PGHOST",
			envVars: map[string]string{
				"PGHOST":     "",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "empty PGUSER",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "empty PGPASSWORD",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "",
				"PGDATABASE": "db",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "empty PGDATABASE",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "",
			},
			expectError: true,
			errorMsg:    "missing PG env vars",
		},
		{
			name: "all required vars present with defaults",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
				// PGPORT and PGSSLMODE use defaults
			},
			expectError: false,
		},
		{
			name: "all required vars present with custom port",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
				"PGPORT":     "5433",
			},
			expectError: false,
		},
		{
			name: "all required vars present with custom sslmode",
			envVars: map[string]string{
				"PGHOST":     "localhost",
				"PGUSER":     "user",
				"PGPASSWORD": "pass",
				"PGDATABASE": "db",
				"PGSSLMODE":  "disable",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all PG env vars
			for k := range originalEnv {
				os.Unsetenv(k)
			}

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			ctx := context.Background()
			_, err := newPoolFromEnv(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("newPoolFromEnv() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("newPoolFromEnv() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				// For validation tests, we only care about validation errors
				// Connection errors are expected when there's no real DB
				if err != nil && strings.Contains(err.Error(), "missing PG env vars") {
					t.Errorf("newPoolFromEnv() unexpected validation error: %v", err)
				}
				// Other errors (like connection errors) are acceptable in this test
			}
		})
	}
}
