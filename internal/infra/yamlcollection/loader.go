package yamlcollection

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	collectionsDir string
}

func NewLoader(opts ...Option) *Loader {
	l := &Loader{collectionsDir: "collections"}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

type Option func(*Loader)

func WithCollectionsDir(dir string) Option {
	return func(l *Loader) { l.collectionsDir = dir }
}

var _ ports.CollectionLoader = (*Loader)(nil)

func (l *Loader) LoadCollection(path string) (domain.Collection, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, &domain.OpError{
			Op:   "yamlcollection.load",
			Kind: domain.KindNotFound,
			Path: path,
			Err:  fmt.Errorf("%w: %w", domain.ErrNotFound, err),
		}
	}

	var yc yamlCollection
	if err := yaml.Unmarshal(b, &yc); err != nil {
		return domain.Collection{}, &domain.OpError{
			Op:   "yamlcollection.load",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  fmt.Errorf("%w: %w", domain.ErrInvalidConfig, err),
		}
	}

	col, err := mapAndValidate(path, yc)
	if err != nil {
		return domain.Collection{}, err
	}

	return col, nil
}

func (l *Loader) ListCollections(root string) ([]domain.CollectionRef, error) {
	dir := filepath.Join(root, l.collectionsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &domain.OpError{
			Op:   "yamlcollection.list",
			Kind: domain.KindNotFound,
			Path: dir,
			Err:  fmt.Errorf("%w: %w", domain.ErrNotFound, err),
		}
	}

	var refs []domain.CollectionRef
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		p := filepath.Join(dir, name)
		n, _ := readCollectionName(p)
		if strings.TrimSpace(n) == "" {
			n = strings.TrimSuffix(name, filepath.Ext(name))
		}

		refs = append(refs, domain.CollectionRef{Name: n, Path: p})
	}

	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs, nil
}

func readCollectionName(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var v struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(b, &v); err != nil {
		return "", err
	}
	return v.Name, nil
}

type yamlCollection struct {
	SchemaVersion *int              `yaml:"schema_version"`
	Name          string            `yaml:"name"`
	Vars          map[string]string `yaml:"vars"`
	Requests      []yamlRequest     `yaml:"requests"`
}

type yamlRequest struct {
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`

	JSON            any               `yaml:"json"`
	Form            map[string]string `yaml:"form"`
	Raw             string            `yaml:"raw"`
	DelayMS         *int              `yaml:"delay_ms"`
	TimeoutMS       *int              `yaml:"timeout_ms"`
	FollowRedirects *bool             `yaml:"follow_redirects"`
	Assert          yamlAssertions    `yaml:"assert"`
	Extract         map[string]string `yaml:"extract"`
	ExtractHeaders  map[string]string `yaml:"extract_headers"`
	Tags            []string          `yaml:"tags"`
}

type yamlAssertions struct {
	Status *int `yaml:"status"`
	MaxMS  *int `yaml:"max_ms"`

	JSONPath     map[string]yamlJSONPathAssertion `yaml:"jsonpath"`
	Headers      map[string]yamlJSONPathAssertion `yaml:"headers"`
	Schema       *string                          `yaml:"schema"`
	SchemaInline map[string]any                   `yaml:"schema_inline"`
}

type yamlJSONPathAssertion struct {
	Exists      bool     `yaml:"exists"`
	Eq          *string  `yaml:"eq"`
	Contains    *string  `yaml:"contains"`
	Matches     *string  `yaml:"matches"`
	Gt          *float64 `yaml:"gt"`
	Lt          *float64 `yaml:"lt"`
	NotEq       *string  `yaml:"not_eq"`
	NotContains *string  `yaml:"not_contains"`
}

func mapAndValidate(path string, yc yamlCollection) (domain.Collection, error) {
	schemaVersion := 1
	if yc.SchemaVersion != nil {
		schemaVersion = *yc.SchemaVersion
	}
	if schemaVersion < 1 {
		return domain.Collection{}, invalidField(path, "schema_version", "must be >= 1")
	}

	if strings.TrimSpace(yc.Name) == "" {
		return domain.Collection{}, invalidField(path, "name", "collection name is required")
	}

	col := domain.Collection{
		SchemaVersion: schemaVersion,
		Name:          yc.Name,
		Vars:          domain.Vars(yc.Vars),
		Requests:      make([]domain.RequestSpec, 0, len(yc.Requests)),
	}

	for i, r := range yc.Requests {
		fieldPrefix := fmt.Sprintf("requests[%d]", i)

		if strings.TrimSpace(r.Name) == "" {
			return domain.Collection{}, invalidField(path, fieldPrefix+".name", "request name is required")
		}
		if strings.TrimSpace(r.URL) == "" {
			return domain.Collection{}, invalidField(path, fieldPrefix+".url", "request url is required")
		}

		method, err := parseMethod(r.Method)
		if err != nil {
			return domain.Collection{}, invalidField(path, fieldPrefix+".method", err.Error())
		}

		if r.Assert.Schema != nil && r.Assert.SchemaInline != nil {
			return domain.Collection{}, invalidField(path, fieldPrefix+".assert",
				"schema and schema_inline cannot be used together")
		}

		// Resolve schema path relative to collection file directory.
		var schemaPtr *string
		if r.Assert.Schema != nil {
			s := *r.Assert.Schema
			if !filepath.IsAbs(s) {
				s = filepath.Join(filepath.Dir(path), s)
			}
			schemaPtr = &s
		}

		req := domain.RequestSpec{
			Name:    r.Name,
			Method:  method,
			URL:     r.URL,
			Headers: domain.Headers(r.Headers),
			Tags:    r.Tags,
			Assert: domain.AssertionsSpec{
				Status:       r.Assert.Status,
				MaxLatencyMS: r.Assert.MaxMS,
				JSONPath:     mapJSONPath(r.Assert.JSONPath),
				Headers:      mapJSONPath(r.Assert.Headers),
				Schema:       schemaPtr,
				SchemaInline: r.Assert.SchemaInline,
			},
			Extract:        domain.ExtractSpec(r.Extract),
			ExtractHeaders: domain.ExtractHeaderSpec(r.ExtractHeaders),
		}

		if req.Headers == nil {
			req.Headers = domain.Headers{}
		}
		if req.Assert.JSONPath == nil {
			req.Assert.JSONPath = map[string]domain.ValueAssertion{}
		}
		if req.Assert.Headers == nil {
			req.Assert.Headers = map[string]domain.ValueAssertion{}
		}
		if req.Extract == nil {
			req.Extract = domain.ExtractSpec{}
		}
		if req.ExtractHeaders == nil {
			req.ExtractHeaders = domain.ExtractHeaderSpec{}
		}

		// Body selection — reject multiple body types.
		bodyCount := 0
		if r.JSON != nil {
			bodyCount++
		}
		if r.Form != nil {
			bodyCount++
		}
		if strings.TrimSpace(r.Raw) != "" {
			bodyCount++
		}
		if bodyCount > 1 {
			return domain.Collection{}, invalidField(path, fieldPrefix+".body",
				"only one body type allowed (json, form, or raw)")
		}

		req.Body = domain.BodySpec{Type: domain.BodyNone}
		if r.JSON != nil {
			if err := domain.ValidateJSONBody(r.JSON); err != nil {
				return domain.Collection{}, invalidField(path, fieldPrefix+".json", err.Error())
			}
			req.Body = domain.BodySpec{Type: domain.BodyJSON, JSON: r.JSON}
		} else if r.Form != nil {
			req.Body = domain.BodySpec{Type: domain.BodyForm, Form: r.Form}
		} else if strings.TrimSpace(r.Raw) != "" {
			req.Body = domain.BodySpec{Type: domain.BodyRaw, Raw: r.Raw}
		}
		if err := req.Body.Validate(); err != nil {
			return domain.Collection{}, invalidField(path, fieldPrefix+".body", err.Error())
		}

		if r.DelayMS != nil && *r.DelayMS < 0 {
			return domain.Collection{}, invalidField(path, fieldPrefix+".delay_ms", "must be >= 0")
		}
		req.DelayMS = r.DelayMS

		if r.TimeoutMS != nil && *r.TimeoutMS <= 0 {
			return domain.Collection{}, invalidField(path, fieldPrefix+".timeout_ms", "must be > 0")
		}
		req.TimeoutMS = r.TimeoutMS
		req.FollowRedirects = r.FollowRedirects

		col.Requests = append(col.Requests, req)
	}

	return col, nil
}

func mapJSONPath(in map[string]yamlJSONPathAssertion) map[string]domain.ValueAssertion {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.ValueAssertion, len(in))
	for k, v := range in {
		out[k] = domain.ValueAssertion{
			Exists:      v.Exists,
			Eq:          v.Eq,
			Contains:    v.Contains,
			Matches:     v.Matches,
			Gt:          v.Gt,
			Lt:          v.Lt,
			NotEq:       v.NotEq,
			NotContains: v.NotContains,
		}
	}
	return out
}

func parseMethod(m string) (domain.HTTPMethod, error) {
	up := strings.ToUpper(strings.TrimSpace(m))
	switch domain.HTTPMethod(up) {
	case domain.MethodGet,
		domain.MethodPost,
		domain.MethodPut,
		domain.MethodPatch,
		domain.MethodDelete,
		domain.MethodHead,
		domain.MethodOptions:
		return domain.HTTPMethod(up), nil
	default:
		return "", fmt.Errorf("unsupported method %q", m)
	}
}

func invalidField(path, field, msg string) error {
	return &domain.OpError{
		Op:   "yamlcollection.validate",
		Kind: domain.KindInvalidConfig,
		Path: path,
		Err:  fmt.Errorf("%w: field %s: %s", domain.ErrInvalidConfig, field, msg),
	}
}
