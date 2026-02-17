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
			Err:  err,
		}
	}

	var yc yamlCollection
	if err := yaml.Unmarshal(b, &yc); err != nil {
		return domain.Collection{}, &domain.OpError{
			Op:   "yamlcollection.load",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
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
			Err:  err,
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
	Name     string            `yaml:"name"`
	Vars     map[string]string `yaml:"vars"`
	Requests []yamlRequest     `yaml:"requests"`
}

type yamlRequest struct {
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`

	JSON        map[string]any    `yaml:"json"`
	Form        map[string]string `yaml:"form"`
	Raw         string            `yaml:"raw"`
	ContentType string            `yaml:"content_type"`

	Assert  yamlAssertions    `yaml:"assert"`
	Extract map[string]string `yaml:"extract"`
}

type yamlAssertions struct {
	Status *int `yaml:"status"`
	MaxMS  *int `yaml:"max_ms"`

	JSONPath map[string]yamlJSONPathAssertion `yaml:"jsonpath"`
}

type yamlJSONPathAssertion struct {
	Exists bool `yaml:"exists"`
}

func mapAndValidate(path string, yc yamlCollection) (domain.Collection, error) {
	if strings.TrimSpace(yc.Name) == "" {
		return domain.Collection{}, invalidField(path, "name", "collection name is required")
	}

	col := domain.Collection{
		Name:     yc.Name,
		Vars:     domain.Vars(yc.Vars),
		Requests: make([]domain.RequestSpec, 0, len(yc.Requests)),
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

		req := domain.RequestSpec{
			Name:    r.Name,
			Method:  method,
			URL:     r.URL,
			Headers: domain.Headers(r.Headers),
			Assert: domain.AssertionsSpec{
				Status:       r.Assert.Status,
				MaxLatencyMS: r.Assert.MaxMS,
				JSONPath:     mapJSONPath(r.Assert.JSONPath),
			},
			Extract: domain.ExtractSpec(r.Extract),
		}

		if req.Headers == nil {
			req.Headers = domain.Headers{}
		}
		if req.Assert.JSONPath == nil {
			req.Assert.JSONPath = map[string]domain.JSONPathAssertion{}
		}
		if req.Extract == nil {
			req.Extract = domain.ExtractSpec{}
		}

		// Body selection
		req.Body = domain.BodySpec{Type: domain.BodyNone}
		if r.JSON != nil {
			req.Body = domain.BodySpec{Type: domain.BodyJSON, JSON: r.JSON}
		} else if r.Form != nil {
			req.Body = domain.BodySpec{Type: domain.BodyForm, Form: r.Form}
		} else if strings.TrimSpace(r.Raw) != "" {
			req.Body = domain.BodySpec{Type: domain.BodyRaw, Raw: r.Raw}
		}
		req.Body.ContentType = strings.TrimSpace(r.ContentType)

		col.Requests = append(col.Requests, req)
	}

	return col, nil
}

func mapJSONPath(in map[string]yamlJSONPathAssertion) map[string]domain.JSONPathAssertion {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.JSONPathAssertion, len(in))
	for k, v := range in {
		out[k] = domain.JSONPathAssertion{Exists: v.Exists}
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
		Err:  fmt.Errorf("field %s: %s", field, msg),
	}
}
