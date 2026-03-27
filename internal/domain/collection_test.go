package domain

import "testing"

func TestCompileDomain(t *testing.T) {
	status := 200
	maxLatency := 150

	col := Collection{
		Name: "sample",
		Vars: Vars{"base_url": "https://example.com"},
		Requests: []RequestSpec{
			{
				Name:   "get user",
				Method: MethodGet,
				URL:    "{{base_url}}/users/1",
				Headers: Headers{
					"Accept": "application/json",
				},
				Body: BodySpec{Type: BodyNone},
				Assert: AssertionsSpec{
					Status:       &status,
					MaxLatencyMS: &maxLatency,
					JSONPath: map[string]ValueAssertion{
						"$.data": {Exists: true},
					},
				},
				Extract: ExtractSpec{
					"user_id": "$.data.id",
				},
			},
		},
	}

	if col.Requests[0].Method != MethodGet {
		t.Fatalf("expected method %s", MethodGet)
	}

	if col.Requests[0].Assert.Status == nil || *col.Requests[0].Assert.Status != status {
		t.Fatalf("expected status %d", status)
	}
}

func TestValidateJSONBody_Object(t *testing.T) {
	err := ValidateJSONBody(map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateJSONBody_Array(t *testing.T) {
	err := ValidateJSONBody([]any{1, 2, 3})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateJSONBody_Nil(t *testing.T) {
	err := ValidateJSONBody(nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateJSONBody_InvalidType(t *testing.T) {
	err := ValidateJSONBody("just a string")
	if err == nil {
		t.Fatal("expected error for string body")
	}
}
