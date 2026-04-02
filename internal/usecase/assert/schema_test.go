package assert

import (
	"strings"
	"testing"
)

func TestSchemaValidate(t *testing.T) {
	simpleSchema := `{
		"type": "object",
		"required": ["id", "name"],
		"properties": {
			"id":   {"type": "integer"},
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		}
	}`

	tests := []struct {
		name       string
		schema     string
		body       string
		wantPass   bool
		wantSubstr string
	}{
		{
			name:     "valid body matches schema",
			schema:   simpleSchema,
			body:     `{"id": 1, "name": "Alice"}`,
			wantPass: true,
		},
		{
			name:     "valid body with extra fields",
			schema:   simpleSchema,
			body:     `{"id": 1, "name": "Alice", "extra": true}`,
			wantPass: true,
		},
		{
			name:       "missing required field",
			schema:     simpleSchema,
			body:       `{"id": 1}`,
			wantPass:   false,
			wantSubstr: "missing property",
		},
		{
			name:       "wrong type for field",
			schema:     simpleSchema,
			body:       `{"id": "not-a-number", "name": "Alice"}`,
			wantPass:   false,
			wantSubstr: "schema validation failed",
		},
		{
			name:       "empty body",
			schema:     simpleSchema,
			body:       ``,
			wantPass:   false,
			wantSubstr: "no body",
		},
		{
			name:       "non-JSON body",
			schema:     simpleSchema,
			body:       `this is not json`,
			wantPass:   false,
			wantSubstr: "not valid JSON",
		},
		{
			name:       "empty schema",
			schema:     ``,
			body:       `{"id": 1}`,
			wantPass:   false,
			wantSubstr: "schema is empty",
		},
		{
			name:       "invalid schema JSON",
			schema:     `{not valid json`,
			body:       `{"id": 1}`,
			wantPass:   false,
			wantSubstr: "invalid schema JSON",
		},
		{
			name:     "array schema",
			schema:   `{"type": "array", "items": {"type": "integer"}}`,
			body:     `[1, 2, 3]`,
			wantPass: true,
		},
		{
			name:       "array schema with wrong item type",
			schema:     `{"type": "array", "items": {"type": "integer"}}`,
			body:       `[1, "two", 3]`,
			wantPass:   false,
			wantSubstr: "schema validation failed",
		},
		{
			name: "nested object schema",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"required": ["id"],
						"properties": {
							"id": {"type": "integer"},
							"role": {"type": "string"}
						}
					}
				}
			}`,
			body:     `{"user": {"id": 42, "role": "admin"}}`,
			wantPass: true,
		},
		{
			name: "nested object missing required",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"required": ["id"],
						"properties": {
							"id": {"type": "integer"}
						}
					}
				}
			}`,
			body:       `{"user": {"role": "admin"}}`,
			wantPass:   false,
			wantSubstr: "missing property",
		},
		{
			name:     "string pattern",
			schema:   `{"type": "object", "properties": {"code": {"type": "string", "pattern": "^[A-Z]{3}$"}}}`,
			body:     `{"code": "ABC"}`,
			wantPass: true,
		},
		{
			name:       "string pattern mismatch",
			schema:     `{"type": "object", "properties": {"code": {"type": "string", "pattern": "^[A-Z]{3}$"}}}`,
			body:       `{"code": "abc"}`,
			wantPass:   false,
			wantSubstr: "pattern",
		},
		{
			name:     "boolean true schema accepts anything",
			schema:   `true`,
			body:     `{"any": "value"}`,
			wantPass: true,
		},
		{
			name:       "boolean false schema rejects everything",
			schema:     `false`,
			body:       `{"any": "value"}`,
			wantPass:   false,
			wantSubstr: "schema validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SchemaValidate([]byte(tt.schema), []byte(tt.body), false)

			if result.Name != "schema" {
				t.Errorf("expected name 'schema', got %q", result.Name)
			}

			if result.Passed != tt.wantPass {
				t.Errorf("passed: got %v, want %v (message: %s)", result.Passed, tt.wantPass, result.Message)
			}

			if tt.wantSubstr != "" && !strings.Contains(result.Message, tt.wantSubstr) {
				t.Errorf("message should contain %q, got %q", tt.wantSubstr, result.Message)
			}
		})
	}
}
