package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

func MapCollection(path string, yc YAMLCollection) (domain.Collection, error) {
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
		if strings.TrimSpace(r.Method) == "" {
			return domain.Collection{}, invalidField(path, fieldPrefix+".method", "method is required")
		}
		if strings.TrimSpace(r.URL) == "" {
			return domain.Collection{}, invalidField(path, fieldPrefix+".url", "url is required")
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

func MapEnvironment(path string, env YAMLEnvironment) (domain.Environment, error) {
	if env.Vars == nil {
		env.Vars = map[string]string{}
	}
	return domain.Environment{
		Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Vars: domain.Vars(env.Vars),
	}, nil
}

func mapJSONPath(in map[string]YAMLJSONPathAssertion) map[string]domain.JSONPathAssertion {
	if in == nil {
		return nil
	}
	out := make(map[string]domain.JSONPathAssertion, len(in))
	for k, v := range in {
		out[k] = domain.JSONPathAssertion{
			Exists:   v.Exists,
			Eq:       v.Eq,
			Contains: v.Contains,
			Matches:  v.Matches,
			Gt:       v.Gt,
			Lt:       v.Lt,
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
		Op:   "config.map",
		Kind: domain.KindInvalidConfig,
		Path: path,
		Err:  fmt.Errorf("field %s: %s: %w", field, msg, domain.ErrInvalidConfig),
	}
}
