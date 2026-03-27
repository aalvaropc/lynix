package domain

import (
	"reflect"
	"testing"
)

func TestBuildDepGraph_Empty(t *testing.T) {
	g := BuildDepGraph(nil, Vars{})
	if len(g.Levels) != 0 {
		t.Errorf("expected 0 levels, got %d", len(g.Levels))
	}
}

func TestBuildDepGraph_AllIndependent(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "a", URL: "http://example.com/a"},
		{Name: "b", URL: "http://example.com/b"},
		{Name: "c", URL: "http://example.com/c"},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 1 {
		t.Fatalf("expected 1 level, got %d: %v", len(g.Levels), g.Levels)
	}
	if !reflect.DeepEqual(g.Levels[0], []int{0, 1, 2}) {
		t.Errorf("expected [0,1,2], got %v", g.Levels[0])
	}
}

func TestBuildDepGraph_LinearChain(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "a", URL: "http://e.com", Extract: ExtractSpec{"x": "$.x"}},
		{Name: "b", URL: "http://e.com/{{x}}", Extract: ExtractSpec{"y": "$.y"}},
		{Name: "c", URL: "http://e.com/{{y}}"},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d: %v", len(g.Levels), g.Levels)
	}
	if !reflect.DeepEqual(g.Levels[0], []int{0}) {
		t.Errorf("level 0: expected [0], got %v", g.Levels[0])
	}
	if !reflect.DeepEqual(g.Levels[1], []int{1}) {
		t.Errorf("level 1: expected [1], got %v", g.Levels[1])
	}
	if !reflect.DeepEqual(g.Levels[2], []int{2}) {
		t.Errorf("level 2: expected [2], got %v", g.Levels[2])
	}
}

func TestBuildDepGraph_Diamond(t *testing.T) {
	// login → (profile, settings) → dashboard
	reqs := []RequestSpec{
		{Name: "login", URL: "http://e.com/login", Extract: ExtractSpec{"token": "$.token"}},
		{Name: "profile", URL: "http://e.com/me", Headers: Headers{"Auth": "{{token}}"}, Extract: ExtractSpec{"name": "$.name"}},
		{Name: "settings", URL: "http://e.com/settings", Headers: Headers{"Auth": "{{token}}"}},
		{Name: "dashboard", URL: "http://e.com/dash/{{name}}"},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 3 {
		t.Fatalf("expected 3 levels, got %d: %v", len(g.Levels), g.Levels)
	}
	if !reflect.DeepEqual(g.Levels[0], []int{0}) {
		t.Errorf("level 0: expected [0], got %v", g.Levels[0])
	}
	if !reflect.DeepEqual(g.Levels[1], []int{1, 2}) {
		t.Errorf("level 1: expected [1,2], got %v", g.Levels[1])
	}
	if !reflect.DeepEqual(g.Levels[2], []int{3}) {
		t.Errorf("level 2: expected [3], got %v", g.Levels[2])
	}
}

func TestBuildDepGraph_BuiltinsNotDeps(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "a", URL: "http://e.com/{{$uuid}}"},
		{Name: "b", URL: "http://e.com/{{$timestamp}}"},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 1 {
		t.Fatalf("expected 1 level (builtins not deps), got %d", len(g.Levels))
	}
}

func TestBuildDepGraph_SeedVarsSatisfy(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "a", URL: "{{base_url}}/path"},
		{Name: "b", URL: "{{base_url}}/other"},
	}
	g := BuildDepGraph(reqs, Vars{"base_url": "http://e.com"})

	if len(g.Levels) != 1 {
		t.Fatalf("expected 1 level (seed vars satisfy), got %d", len(g.Levels))
	}
}

func TestBuildDepGraph_BodyVarRefs(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "login", URL: "http://e.com", Extract: ExtractSpec{"token": "$.t"}},
		{Name: "create", URL: "http://e.com", Body: BodySpec{
			Type: BodyJSON,
			JSON: map[string]any{"auth": "{{token}}"},
		}},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 2 {
		t.Fatalf("expected 2 levels (body ref), got %d: %v", len(g.Levels), g.Levels)
	}
}

func TestBuildDepGraph_FormBodyVarRefs(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "auth", URL: "http://e.com", Extract: ExtractSpec{"key": "$.k"}},
		{Name: "submit", URL: "http://e.com", Body: BodySpec{
			Type: BodyForm,
			Form: map[string]string{"api_key": "{{key}}"},
		}},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(g.Levels))
	}
}

func TestBuildDepGraph_ExtractHeaders(t *testing.T) {
	reqs := []RequestSpec{
		{Name: "login", URL: "http://e.com", ExtractHeaders: ExtractHeaderSpec{"cookie": "Set-Cookie"}},
		{Name: "profile", URL: "http://e.com", Headers: Headers{"Cookie": "{{cookie}}"}},
	}
	g := BuildDepGraph(reqs, Vars{})

	if len(g.Levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(g.Levels))
	}
}

func TestExtractVarRefs_Basic(t *testing.T) {
	refs := extractVarRefs("{{base_url}}/users/{{user_id}}")
	if !reflect.DeepEqual(refs, []string{"base_url", "user_id"}) {
		t.Errorf("got %v", refs)
	}
}

func TestExtractVarRefs_SkipsBuiltins(t *testing.T) {
	refs := extractVarRefs("{{$uuid}}/{{name}}")
	if !reflect.DeepEqual(refs, []string{"name"}) {
		t.Errorf("expected [name], got %v", refs)
	}
}

func TestExtractVarRefs_NoPlaceholders(t *testing.T) {
	refs := extractVarRefs("http://example.com/path")
	if len(refs) != 0 {
		t.Errorf("expected empty, got %v", refs)
	}
}
