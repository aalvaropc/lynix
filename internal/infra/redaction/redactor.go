package redaction

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

const maskValue = "********"

// builtinHeaderPatterns are always masked (case-insensitive substring match on header key).
var builtinHeaderPatterns = []string{
	"authorization", "proxy-authorization", "cookie", "set-cookie",
	"x-api-key", "x-auth-token",
	"token", "secret", "password", "api-key", "apikey",
}

// builtinKeyPatterns are always masked for body fields, query params, and extracted vars.
var builtinKeyPatterns = []string{
	"token", "secret", "password", "api_key", "apikey",
	"api-key", "access_key", "private_key", "credential",
}

// Redactor masks sensitive data across all surfaces of a RunArtifact.
type Redactor struct {
	cfg domain.MaskingConfig
}

// New creates a Redactor from a MaskingConfig.
func New(cfg domain.MaskingConfig) *Redactor {
	return &Redactor{cfg: cfg}
}

// Redact returns a deep copy of the run artifact with sensitive data masked.
// It does NOT mutate the input.
func (r *Redactor) Redact(run domain.RunArtifact) domain.RunArtifact {
	if !r.cfg.Enabled {
		return run
	}

	out := run
	out.Results = make([]domain.RequestResult, 0, len(run.Results))

	for _, rr := range run.Results {
		c := rr

		// Request headers
		if r.cfg.MaskRequestHeaders && len(rr.RequestHeaders) > 0 {
			c.RequestHeaders = r.maskStringMap(rr.RequestHeaders, r.isHeaderSensitive)
		}

		// Response headers
		if len(rr.Response.Headers) > 0 {
			c.Response = cloneResponseSnapshot(rr.Response)
			for k := range c.Response.Headers {
				if r.isHeaderSensitive(k) {
					vals := c.Response.Headers[k]
					masked := make([]string, len(vals))
					for i := range vals {
						masked[i] = maskValue
					}
					c.Response.Headers[k] = masked
				}
			}
		}

		// Request body (JSON fields)
		if r.cfg.MaskRequestBody && len(rr.RequestBody) > 0 {
			c.RequestBody = r.maskJSONBytes(rr.RequestBody)
		}

		// Response body (JSON fields)
		if r.cfg.MaskResponseBody && len(rr.Response.Body) > 0 {
			snap := cloneResponseSnapshot(c.Response)
			snap.Body = r.maskJSONBytes(rr.Response.Body)
			c.Response = snap
		}

		// Query params in resolved URL
		if r.cfg.MaskQueryParams && rr.ResolvedURL != "" {
			c.ResolvedURL = r.maskQueryParams(rr.ResolvedURL)
		}

		// Extracted vars
		if len(rr.Extracted) > 0 {
			c.Extracted = r.maskStringMap(cloneVars(rr.Extracted), r.isKeySensitive)
		}

		// Deep copy slices that might be shared
		c.Extracts = cloneExtractResults(rr.Extracts)
		c.Assertions = cloneAssertionResults(rr.Assertions)

		out.Results = append(out.Results, c)
	}

	return out
}

func (r *Redactor) isHeaderSensitive(key string) bool {
	kk := strings.ToLower(strings.TrimSpace(key))
	for _, p := range builtinHeaderPatterns {
		if strings.Contains(kk, p) {
			return true
		}
	}
	for _, rule := range r.cfg.Rules {
		if rule.Scope != domain.RedactionScopeAll && rule.Scope != domain.RedactionScopeHeader {
			continue
		}
		if strings.Contains(kk, strings.ToLower(rule.Pattern)) {
			return true
		}
	}
	return false
}

func (r *Redactor) isKeySensitive(key string) bool {
	kk := strings.ToLower(strings.TrimSpace(key))
	for _, p := range builtinKeyPatterns {
		if strings.Contains(kk, p) {
			return true
		}
	}
	for _, rule := range r.cfg.Rules {
		if rule.Scope != domain.RedactionScopeAll &&
			rule.Scope != domain.RedactionScopeBody &&
			rule.Scope != domain.RedactionScopeQuery {
			continue
		}
		if strings.Contains(kk, strings.ToLower(rule.Pattern)) {
			return true
		}
	}
	return false
}

func (r *Redactor) isQueryParamSensitive(key string) bool {
	kk := strings.ToLower(strings.TrimSpace(key))
	for _, p := range builtinKeyPatterns {
		if strings.Contains(kk, p) {
			return true
		}
	}
	for _, rule := range r.cfg.Rules {
		if rule.Scope != domain.RedactionScopeAll && rule.Scope != domain.RedactionScopeQuery {
			continue
		}
		if strings.Contains(kk, strings.ToLower(rule.Pattern)) {
			return true
		}
	}
	return false
}

func (r *Redactor) maskStringMap(m map[string]string, isSensitive func(string) bool) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if isSensitive(k) {
			out[k] = maskValue
		} else {
			out[k] = v
		}
	}
	return out
}

func (r *Redactor) maskQueryParams(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	changed := false
	for k := range q {
		if r.isQueryParamSensitive(k) {
			q.Set(k, maskValue)
			changed = true
		}
	}
	if !changed {
		return rawURL
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// maskJSONBytes attempts to mask sensitive keys in a JSON document.
// If the body is not valid JSON, it is returned as-is.
func (r *Redactor) maskJSONBytes(body []byte) []byte {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return body // not JSON, return as-is
	}
	r.walkAndMask(doc)
	masked, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return masked
}

func (r *Redactor) walkAndMask(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if r.isKeySensitive(k) {
				t[k] = maskValue
			} else {
				r.walkAndMask(val)
			}
		}
	case []any:
		for _, item := range t {
			r.walkAndMask(item)
		}
	}
}

// --- deep copy helpers (migrated from runstore) ---

func cloneVars(in domain.Vars) domain.Vars {
	if in == nil {
		return domain.Vars{}
	}
	out := domain.Vars{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneExtractResults(in []domain.ExtractResult) []domain.ExtractResult {
	if in == nil {
		return []domain.ExtractResult{}
	}
	out := make([]domain.ExtractResult, len(in))
	copy(out, in)
	return out
}

func cloneAssertionResults(in []domain.AssertionResult) []domain.AssertionResult {
	if in == nil {
		return []domain.AssertionResult{}
	}
	out := make([]domain.AssertionResult, len(in))
	copy(out, in)
	return out
}

func cloneResponseSnapshot(in domain.ResponseSnapshot) domain.ResponseSnapshot {
	out := domain.ResponseSnapshot{
		Truncated: in.Truncated,
	}
	if in.Headers != nil {
		out.Headers = make(map[string][]string, len(in.Headers))
		for k, v := range in.Headers {
			cp := make([]string, len(v))
			copy(cp, v)
			out.Headers[k] = cp
		}
	} else {
		out.Headers = map[string][]string{}
	}
	if in.Body != nil {
		out.Body = make([]byte, len(in.Body))
		copy(out.Body, in.Body)
	}
	return out
}
