package redaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// ErrSecretDetected is returned when an unmasked secret is found in an artifact.
var ErrSecretDetected = errors.New("detected unmasked secret in artifact")

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

// secretValuePatterns match well-known credential formats regardless of the
// key they appear under (JWTs, GitHub/Stripe/AWS/Slack tokens).
var secretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`sk_(?:live|test)_[A-Za-z0-9]{10,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
}

// urlInTextPattern finds URLs embedded in free text (e.g. Go's *url.Error
// messages, which include the full request URL with its query string).
var urlInTextPattern = regexp.MustCompile(`https?://[^\s"']+`)

// minSecretValueLen avoids over-masking on trivially short values.
const minSecretValueLen = 4

// Redactor masks sensitive data across all surfaces of a RunArtifact.
type Redactor struct {
	cfg          domain.MaskingConfig
	secretValues []string
}

// New creates a Redactor from a MaskingConfig.
func New(cfg domain.MaskingConfig) *Redactor {
	return &Redactor{cfg: cfg}
}

// AddSecretValues registers literal values to scrub from every surface,
// including free text (error messages, assertion messages, URLs, bodies).
func (r *Redactor) AddSecretValues(vals ...string) {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if len(v) >= minSecretValueLen {
			r.secretValues = append(r.secretValues, v)
		}
	}
}

// AddSecretsFromEnv registers every value from the environment's secrets file
// plus any env var whose name looks sensitive. Known values are the most
// reliable redaction signal: they are matched literally on all surfaces.
func (r *Redactor) AddSecretsFromEnv(env domain.Environment) {
	r.AddSecretValues(env.SecretValues...)
	for k, v := range env.Vars {
		if r.isKeySensitive(k) {
			r.AddSecretValues(v)
		}
	}
}

// scrubText replaces known secret values and well-known credential formats.
func (r *Redactor) scrubText(s string) string {
	if s == "" {
		return s
	}
	for _, v := range r.secretValues {
		s = strings.ReplaceAll(s, v, maskValue)
	}
	for _, p := range secretValuePatterns {
		s = p.ReplaceAllString(s, maskValue)
	}
	return s
}

func (r *Redactor) scrubBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte(r.scrubText(string(b)))
}

// maskURLsInText applies query-param masking to every URL found in free text.
func (r *Redactor) maskURLsInText(s string) string {
	return urlInTextPattern.ReplaceAllStringFunc(s, r.maskQueryParams)
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

		// URL surfaces. URL and ResolvedURL carry the same resolved value in
		// practice, so both must be masked (URL used to leak query secrets).
		if r.cfg.MaskQueryParams {
			c.URL = r.maskQueryParams(c.URL)
			c.ResolvedURL = r.maskQueryParams(c.ResolvedURL)
		}
		c.URL = r.scrubText(c.URL)
		c.ResolvedURL = r.scrubText(c.ResolvedURL)

		// Request headers: key-based masking per config, value scrub always.
		c.RequestHeaders = r.maskStringMap(rr.RequestHeaders, r.cfg.MaskRequestHeaders, r.isHeaderSensitive)

		// Response headers.
		c.Response = cloneResponseSnapshot(rr.Response)
		for k, vals := range c.Response.Headers {
			for i := range vals {
				if r.cfg.MaskResponseHeaders && r.isHeaderSensitive(k) {
					vals[i] = maskValue
				} else {
					vals[i] = r.scrubText(vals[i])
				}
			}
		}

		// Bodies: key-based masking for JSON/form per config, value scrub always.
		if r.cfg.MaskRequestBody {
			c.RequestBody = r.maskBodyBytes(rr.RequestBody)
		} else {
			c.RequestBody = r.scrubBytes(rr.RequestBody)
		}
		if r.cfg.MaskResponseBody {
			c.Response.Body = r.maskBodyBytes(c.Response.Body)
		} else {
			c.Response.Body = r.scrubBytes(c.Response.Body)
		}

		// Extracted vars: key-based masking + value scrub.
		c.Extracted = r.maskStringMap(rr.Extracted, true, r.isKeySensitive)

		// Assertion and extract messages embed observed response values
		// (e.g. `expected "x", got "<token>"`), so they are scrubbed too.
		c.Extracts = cloneExtractResults(rr.Extracts)
		for i := range c.Extracts {
			c.Extracts[i].Message = r.scrubText(c.Extracts[i].Message)
		}
		c.Assertions = cloneAssertionResults(rr.Assertions)
		for i := range c.Assertions {
			c.Assertions[i].Message = r.scrubText(c.Assertions[i].Message)
		}

		// Error messages wrap the full request URL (Go's *url.Error), which
		// leaks query-string secrets into artifacts, stdout, and JUnit XML.
		if rr.Error != nil {
			e := *rr.Error
			e.Message = r.scrubText(r.maskURLsInText(e.Message))
			c.Error = &e
		}

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

func (r *Redactor) maskStringMap(m map[string]string, keyMasking bool, isSensitive func(string) bool) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if keyMasking && isSensitive(k) {
			out[k] = maskValue
		} else {
			out[k] = r.scrubText(v)
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

// maskBodyBytes masks sensitive keys in JSON or form-urlencoded bodies and
// falls back to a value scrub for everything else (XML, plain text, ...).
// A body must never pass through untouched: form logins used to leak
// password=... verbatim because only JSON was handled.
func (r *Redactor) maskBodyBytes(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	if masked, ok := r.maskJSONBody(body); ok {
		return masked
	}
	if masked, ok := r.maskFormBody(body); ok {
		return masked
	}
	return r.scrubBytes(body)
}

func (r *Redactor) maskJSONBody(body []byte) ([]byte, bool) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false
	}
	r.walkAndMask(doc)
	masked, err := json.Marshal(doc)
	if err != nil {
		return nil, false
	}
	return r.scrubBytes(masked), true
}

// maskFormBody handles urlencoded-style bodies (k=v&k=v).
func (r *Redactor) maskFormBody(body []byte) ([]byte, bool) {
	s := string(body)
	if !strings.Contains(s, "=") || strings.ContainsAny(s, " \t\n{") {
		return nil, false
	}
	vals, err := url.ParseQuery(s)
	if err != nil {
		return nil, false
	}
	for k, vv := range vals {
		if r.isKeySensitive(k) {
			vals[k] = []string{maskValue}
			continue
		}
		for i := range vv {
			vv[i] = r.scrubText(vv[i])
		}
	}
	return []byte(vals.Encode()), true
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

// CheckForSecrets scans a (presumably already-redacted) RunArtifact for any
// sensitive values that were NOT masked. Returns ErrSecretDetected on first hit.
//
// Unlike Redact, this is not limited to the same key-based predicates (which
// would make the check circular): it also scans the fully serialized artifact
// for known secret values and well-known credential formats.
func (r *Redactor) CheckForSecrets(run domain.RunArtifact) error {
	if serialized, err := json.Marshal(run); err == nil {
		if err := r.checkTextForSecrets(string(serialized)); err != nil {
			return err
		}
	}
	// Bodies serialize as base64 in JSON, so they need a separate raw scan.
	for _, rr := range run.Results {
		if err := r.checkTextForSecrets(string(rr.RequestBody)); err != nil {
			return fmt.Errorf("%w (request body of %q)", err, rr.Name)
		}
		if err := r.checkTextForSecrets(string(rr.Response.Body)); err != nil {
			return fmt.Errorf("%w (response body of %q)", err, rr.Name)
		}
	}

	for _, rr := range run.Results {
		// Request headers
		for k, v := range rr.RequestHeaders {
			if r.isHeaderSensitive(k) && v != maskValue {
				return fmt.Errorf("%w: request header %q in request %q", ErrSecretDetected, k, rr.Name)
			}
		}

		// Response headers
		for k, vals := range rr.Response.Headers {
			if r.isHeaderSensitive(k) {
				for _, v := range vals {
					if v != maskValue {
						return fmt.Errorf("%w: response header %q in request %q", ErrSecretDetected, k, rr.Name)
					}
				}
			}
		}

		// Request body JSON keys
		if err := r.checkJSONSecrets(rr.RequestBody, "request body", rr.Name); err != nil {
			return err
		}

		// Response body JSON keys
		if err := r.checkJSONSecrets(rr.Response.Body, "response body", rr.Name); err != nil {
			return err
		}

		// Query params in both URL surfaces
		if rr.URL != "" {
			if err := r.checkQuerySecrets(rr.URL, rr.Name); err != nil {
				return err
			}
		}
		if rr.ResolvedURL != "" {
			if err := r.checkQuerySecrets(rr.ResolvedURL, rr.Name); err != nil {
				return err
			}
		}

		// Extracted vars
		for k, v := range rr.Extracted {
			if r.isKeySensitive(k) && v != maskValue {
				return fmt.Errorf("%w: extracted var %q in request %q", ErrSecretDetected, k, rr.Name)
			}
		}
	}
	return nil
}

// checkTextForSecrets scans free text for known secret values and well-known
// credential formats.
func (r *Redactor) checkTextForSecrets(s string) error {
	if s == "" {
		return nil
	}
	for _, v := range r.secretValues {
		if strings.Contains(s, v) {
			return fmt.Errorf("%w: a known secret value appears unmasked in the artifact", ErrSecretDetected)
		}
	}
	for _, p := range secretValuePatterns {
		if p.MatchString(s) {
			return fmt.Errorf("%w: a value matching a known credential format appears in the artifact", ErrSecretDetected)
		}
	}
	return nil
}

func (r *Redactor) checkJSONSecrets(body []byte, surface, reqName string) error {
	if len(body) == 0 {
		return nil
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil // not JSON
	}
	if key := r.findUnmaskedKey(doc); key != "" {
		return fmt.Errorf("%w: %s key %q in request %q", ErrSecretDetected, surface, key, reqName)
	}
	return nil
}

// findUnmaskedKey walks a JSON document and returns the first sensitive key
// whose value is not the mask placeholder. Returns "" if clean.
func (r *Redactor) findUnmaskedKey(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if r.isKeySensitive(k) {
				if s, ok := val.(string); !ok || s != maskValue {
					return k
				}
			} else {
				if found := r.findUnmaskedKey(val); found != "" {
					return found
				}
			}
		}
	case []any:
		for _, item := range t {
			if found := r.findUnmaskedKey(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func (r *Redactor) checkQuerySecrets(rawURL, reqName string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	for k, vals := range u.Query() {
		if r.isQueryParamSensitive(k) {
			for _, v := range vals {
				if v != maskValue {
					return fmt.Errorf("%w: query param %q in request %q", ErrSecretDetected, k, reqName)
				}
			}
		}
	}
	return nil
}

// --- deep copy helpers ---

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
