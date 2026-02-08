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
					JSONPath: map[string]JSONPathAssertion{
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
