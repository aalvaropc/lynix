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

func TestMapCollectionJSONPathExtendedAssertions(t *testing.T) {
	eq := "alice"
	contains := "ali"
	matches := "^[a-z]+$"
	gt := 0.0
	lt := 100.0

	col := YAMLCollection{
		Name: "extended",
		Requests: []YAMLRequest{
			{
				Name:   "check",
				Method: "GET",
				URL:    "https://example.com",
				Assert: YAMLAssertions{
					JSONPath: map[string]YAMLJSONPathAssertion{
						"$.name": {
							Exists:   true,
							Eq:       &eq,
							Contains: &contains,
							Matches:  &matches,
						},
						"$.count": {
							Gt: &gt,
							Lt: &lt,
						},
					},
				},
			},
		},
	}

	mapped, err := MapCollection("collection.yaml", col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := mapped.Requests[0]

	name := req.Assert.JSONPath["$.name"]
	if !name.Exists {
		t.Error("expected Exists=true for $.name")
	}
	if name.Eq == nil || *name.Eq != eq {
		t.Errorf("expected Eq=%q, got %v", eq, name.Eq)
	}
	if name.Contains == nil || *name.Contains != contains {
		t.Errorf("expected Contains=%q, got %v", contains, name.Contains)
	}
	if name.Matches == nil || *name.Matches != matches {
		t.Errorf("expected Matches=%q, got %v", matches, name.Matches)
	}

	count := req.Assert.JSONPath["$.count"]
	if count.Gt == nil || *count.Gt != gt {
		t.Errorf("expected Gt=%v, got %v", gt, count.Gt)
	}
	if count.Lt == nil || *count.Lt != lt {
		t.Errorf("expected Lt=%v, got %v", lt, count.Lt)
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
