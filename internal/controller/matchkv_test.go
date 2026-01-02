package controller

import (
	"testing"
)

func TestMatchKV(t *testing.T) {
	tests := []struct {
		name     string
		lbls     map[string]string
		sel      string
		expected bool
	}{
		{
			name:     "empty selector matches any labels",
			lbls:     map[string]string{"app": "test"},
			sel:      "",
			expected: true,
		},
		{
			name:     "single key-value match",
			lbls:     map[string]string{"app": "test"},
			sel:      "app=test",
			expected: true,
		},
		{
			name:     "single key-value mismatch",
			lbls:     map[string]string{"app": "test"},
			sel:      "app=other",
			expected: false,
		},
		{
			name:     "key exists but value different",
			lbls:     map[string]string{"app": "test"},
			sel:      "app=prod",
			expected: false,
		},
		{
			name:     "key doesn't exist",
			lbls:     map[string]string{"app": "test"},
			sel:      "env=prod",
			expected: false,
		},
		{
			name:     "multiple key-value pairs all match",
			lbls:     map[string]string{"app": "test", "env": "dev", "tier": "frontend"},
			sel:      "app=test,env=dev",
			expected: true,
		},
		{
			name:     "multiple key-value pairs one mismatch",
			lbls:     map[string]string{"app": "test", "env": "dev"},
			sel:      "app=test,env=prod",
			expected: false,
		},
		{
			name:     "multiple key-value pairs with spaces",
			lbls:     map[string]string{"app": "test", "env": "dev"},
			sel:      "app=test, env=dev",
			expected: true,
		},
		{
			name:     "multiple key-value pairs with extra spaces",
			lbls:     map[string]string{"app": "test", "env": "dev"},
			sel:      "app=test , env=dev ",
			expected: true,
		},
		{
			name:     "empty labels with non-empty selector",
			lbls:     map[string]string{},
			sel:      "app=test",
			expected: false,
		},
		{
			name:     "nil labels with non-empty selector",
			lbls:     nil,
			sel:      "app=test",
			expected: false,
		},
		{
			name:     "invalid selector format - no equals",
			lbls:     map[string]string{"app": "test"},
			sel:      "app",
			expected: false,
		},
		{
			name:     "selector with empty value",
			lbls:     map[string]string{"app": ""},
			sel:      "app=",
			expected: true,
		},
		{
			name:     "selector with empty value but label has value",
			lbls:     map[string]string{"app": "test"},
			sel:      "app=",
			expected: false,
		},
		{
			name:     "empty pairs in selector are skipped",
			lbls:     map[string]string{"app": "test"},
			sel:      ",app=test,",
			expected: true,
		},
		{
			name:     "selector with only commas",
			lbls:     map[string]string{"app": "test"},
			sel:      ",,,",
			expected: true,
		},
		{
			name:     "value with special characters",
			lbls:     map[string]string{"app": "test-v1.2.3"},
			sel:      "app=test-v1.2.3",
			expected: true,
		},
		{
			name:     "value with equals sign in value",
			lbls:     map[string]string{"config": "key=value"},
			sel:      "config=key=value",
			expected: true, // SplitN splits on first =, so "key=value" is the value
		},
		{
			name:     "invalid selector format - multiple equals with mismatch",
			lbls:     map[string]string{"app": "test"},
			sel:      "app=test=value",
			expected: false, // Label value is "test", not "test=value"
		},
		{
			name:     "case sensitive matching",
			lbls:     map[string]string{"App": "Test"},
			sel:      "app=test",
			expected: false,
		},
		{
			name:     "three key-value pairs all match",
			lbls:     map[string]string{"app": "test", "env": "dev", "tier": "frontend"},
			sel:      "app=test,env=dev,tier=frontend",
			expected: true,
		},
		{
			name:     "three key-value pairs one mismatch",
			lbls:     map[string]string{"app": "test", "env": "dev", "tier": "frontend"},
			sel:      "app=test,env=dev,tier=backend",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchKV(tt.lbls, tt.sel)
			if result != tt.expected {
				t.Errorf("matchKV(%v, %q) = %v, want %v", tt.lbls, tt.sel, result, tt.expected)
			}
		})
	}
}
