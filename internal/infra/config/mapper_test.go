package config

import (
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestMapCollectionRequiresMethodAndURL(t *testing.T) {
	col := YAMLCollection{
		Name: "sample",
		Requests: []YAMLRequest{
			{
				Name: "missing",
				URL:  "",
			},
		},
	}

	_, err := MapCollection("collection.yaml", col)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requests[0].method") {
		t.Fatalf("expected method error, got %v", err)
	}

	col.Requests[0].Method = "GET"
	_, err = MapCollection("collection.yaml", col)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requests[0].url") {
		t.Fatalf("expected url error, got %v", err)
	}
}

func TestMapCollectionBodyAndAssertions(t *testing.T) {
	status := 200
	maxMS := 150

	col := YAMLCollection{
		Name: "sample",
		Requests: []YAMLRequest{
			{
				Name:   "get",
				Method: "GET",
				URL:    "https://example.com",
				JSON: map[string]any{
					"a": "b",
				},
				Assert: YAMLAssertions{
					Status: &status,
					MaxMS:  &maxMS,
					JSONPath: map[string]YAMLJSONPathAssertion{
						"$.data": {Exists: true},
					},
				},
				Extract: map[string]string{
					"token": "$.token",
				},
			},
		},
	}

	mapped, err := MapCollection("collection.yaml", col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := mapped.Requests[0]
	if req.Method != domain.MethodGet {
		t.Fatalf("expected method GET")
	}
	if req.Body.Type != domain.BodyJSON {
		t.Fatalf("expected JSON body")
	}
	if req.Assert.Status == nil || *req.Assert.Status != status {
		t.Fatalf("expected status to map")
	}
	if req.Assert.MaxLatencyMS == nil || *req.Assert.MaxLatencyMS != maxMS {
		t.Fatalf("expected max latency to map")
	}
	if !req.Assert.JSONPath["$.data"].Exists {
		t.Fatalf("expected jsonpath exists to map")
	}
	if req.Extract["token"] != "$.token" {
		t.Fatalf("expected extract to map")
	}
}

func TestMapEnvironmentDefaultsVars(t *testing.T) {
	env, err := MapEnvironment("env/dev.yaml", YAMLEnvironment{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "dev" {
		t.Fatalf("expected env name dev, got %q", env.Name)
	}
	if env.Vars == nil {
		t.Fatalf("expected vars to be initialized")
	}
}
