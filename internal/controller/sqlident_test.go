package controller

import (
	"testing"
)

func TestSanitizeTableIdent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string defaults to public.server",
			input:    "",
			expected: `"public"."server"`,
		},
		{
			name:     "simple table name",
			input:    "server",
			expected: `"server"`,
		},
		{
			name:     "schema.table format",
			input:    "public.server",
			expected: `"public"."server"`,
		},
		{
			name:     "custom schema and table",
			input:    "myschema.mytable",
			expected: `"myschema"."mytable"`,
		},
		{
			name:     "table name with underscores",
			input:    "my_table",
			expected: `"my_table"`,
		},
		{
			name:     "table name with numbers",
			input:    "table123",
			expected: `"table123"`,
		},
		{
			name:     "schema and table with underscores",
			input:    "my_schema.my_table",
			expected: `"my_schema"."my_table"`,
		},
		{
			name:     "table name with mixed case",
			input:    "MyTable",
			expected: `"MyTable"`,
		},
		{
			name:     "schema and table with mixed case",
			input:    "MySchema.MyTable",
			expected: `"MySchema"."MyTable"`,
		},
		{
			name:     "three part identifier (schema.schema.table)",
			input:    "public.myschema.server",
			expected: `"public"."myschema"."server"`,
		},
		{
			name:     "table name starting with number",
			input:    "123table",
			expected: `"123table"`,
		},
		{
			name:     "table name with special characters",
			input:    "my-table",
			expected: `"my-table"`,
		},
		{
			name:     "schema and table with special characters",
			input:    "my-schema.my-table",
			expected: `"my-schema"."my-table"`,
		},
		{
			name:     "table name with spaces (should be quoted)",
			input:    "my table",
			expected: `"my table"`,
		},
		{
			name:     "table name with reserved words",
			input:    "select",
			expected: `"select"`,
		},
		{
			name:     "schema and table with reserved words",
			input:    "public.select",
			expected: `"public"."select"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTableIdent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTableIdent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
