package domain

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// VarResolver resolves {{var}} placeholders in strings and JSON-like payloads.
// Built-ins: {{$timestamp}}, {{$uuid}}, {{$isoTimestamp}}, {{$randomInt}},
// {{$randomString}}, {{$randomEmail}}, {{$randomBool}}.
//
// This lives in domain because it does not depend on YAML/FS/HTTP. Only stdlib.
type VarResolver struct {
	now        func() time.Time
	uuidV4     func() (string, error)
	randSource io.Reader
}

// VarResolverOption configures VarResolver.
type VarResolverOption func(*VarResolver)

// WithNow overrides the clock (useful for tests).
func WithNow(now func() time.Time) VarResolverOption {
	return func(r *VarResolver) { r.now = now }
}

// WithUUID overrides UUID generation (useful for tests).
func WithUUID(gen func() (string, error)) VarResolverOption {
	return func(r *VarResolver) { r.uuidV4 = gen }
}

// WithRand overrides the random source (useful for deterministic tests).
func WithRand(src io.Reader) VarResolverOption {
	return func(r *VarResolver) { r.randSource = src }
}

func NewVarResolver(opts ...VarResolverOption) *VarResolver {
	r := &VarResolver{
		now:        time.Now,
		uuidV4:     uuidV4,
		randSource: rand.Reader,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RuntimeResolver caches built-ins for a single "resolution session" (e.g., one request run)
// so repeated {{$uuid}} inside multiple fields stays consistent.
type RuntimeResolver struct {
	base     Vars
	builtins Vars
	inner    *VarResolver
}

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func (r *VarResolver) NewRuntime(vars Vars) (*RuntimeResolver, error) {
	now := r.now()
	ts := strconv.FormatInt(now.Unix(), 10)
	isoTS := now.UTC().Format(time.RFC3339)

	u, err := r.uuidV4()
	if err != nil {
		return nil, &OpError{Op: "vars.builtins.uuid", Kind: KindExecution, Err: err}
	}

	randomInt, randomStr, randomEmail, randomBool, err := r.generateRandomBuiltins()
	if err != nil {
		return nil, err
	}

	baseCopy := Vars{}
	for k, v := range vars {
		baseCopy[k] = v
	}

	return &RuntimeResolver{
		base: baseCopy,
		builtins: Vars{
			"$timestamp":    ts,
			"$uuid":         u,
			"$isoTimestamp": isoTS,
			"$randomInt":    randomInt,
			"$randomString": randomStr,
			"$randomEmail":  randomEmail,
			"$randomBool":   randomBool,
		},
		inner: r,
	}, nil
}

func (r *VarResolver) generateRandomBuiltins() (rInt, rStr, rEmail, rBool string, err error) {
	// $randomInt: 0-9999
	var intBuf [2]byte
	if _, err = io.ReadFull(r.randSource, intBuf[:]); err != nil {
		return "", "", "", "", &OpError{Op: "vars.builtins.random", Kind: KindExecution, Err: err}
	}
	rInt = strconv.Itoa(int(binary.BigEndian.Uint16(intBuf[:])) % 10000)

	// $randomString: 8 alphanumeric chars
	var strBuf [8]byte
	if _, err = io.ReadFull(r.randSource, strBuf[:]); err != nil {
		return "", "", "", "", &OpError{Op: "vars.builtins.random", Kind: KindExecution, Err: err}
	}
	var sb strings.Builder
	sb.Grow(8)
	for _, b := range strBuf {
		sb.WriteByte(alphanumeric[int(b)%len(alphanumeric)])
	}
	rStr = sb.String()

	// $randomEmail: user_<6chars>@test.lynix
	var emailBuf [6]byte
	if _, err = io.ReadFull(r.randSource, emailBuf[:]); err != nil {
		return "", "", "", "", &OpError{Op: "vars.builtins.random", Kind: KindExecution, Err: err}
	}
	var eb strings.Builder
	eb.Grow(22)
	eb.WriteString("user_")
	for _, b := range emailBuf {
		eb.WriteByte(alphanumeric[int(b)%len(alphanumeric)])
	}
	eb.WriteString("@test.lynix")
	rEmail = eb.String()

	// $randomBool: "true" or "false"
	var boolBuf [1]byte
	if _, err = io.ReadFull(r.randSource, boolBuf[:]); err != nil {
		return "", "", "", "", &OpError{Op: "vars.builtins.random", Kind: KindExecution, Err: err}
	}
	rBool = "false"
	if boolBuf[0]%2 == 1 {
		rBool = "true"
	}

	return rInt, rStr, rEmail, rBool, nil
}

// ResolveString resolves placeholders in a string.
// Supported tokens: {{base_url}}, {{token}}, {{$timestamp}}, {{$uuid}}.
func (rr *RuntimeResolver) ResolveString(s string) (string, error) {
	return rr.inner.resolveStringWith(rr.base, rr.builtins, s)
}

// ResolveHeaders resolves placeholders in header values.
func (rr *RuntimeResolver) ResolveHeaders(h Headers) (Headers, error) {
	out := Headers{}
	for k, v := range h {
		rv, err := rr.ResolveString(v)
		if err != nil {
			return nil, err
		}
		out[k] = rv
	}
	return out, nil
}

// ResolveBodySpec resolves placeholders inside the body.
// - JSON: resolves ONLY string values recursively (maps/slices supported)
// - Form: resolves values
// - Raw: resolves the raw string
func (rr *RuntimeResolver) ResolveBodySpec(b BodySpec) (BodySpec, error) {
	out := b

	switch b.Type {
	case BodyJSON:
		if b.JSON == nil {
			out.JSON = nil
			return out, nil
		}
		clone, err := rr.ResolveJSONValue(b.JSON)
		if err != nil {
			return BodySpec{}, err
		}
		out.JSON = clone
		return out, nil

	case BodyForm:
		if b.Form == nil {
			out.Form = nil
			return out, nil
		}
		f := map[string]string{}
		for k, v := range b.Form {
			rv, err := rr.ResolveString(v)
			if err != nil {
				return BodySpec{}, err
			}
			f[k] = rv
		}
		out.Form = f
		return out, nil

	case BodyRaw:
		rv, err := rr.ResolveString(b.Raw)
		if err != nil {
			return BodySpec{}, err
		}
		out.Raw = rv
		return out, nil

	default:
		return out, nil
	}
}

// ResolveRequest resolves placeholders in URL, headers and body.
// It returns a copy (does not mutate input).
func (rr *RuntimeResolver) ResolveRequest(req RequestSpec) (RequestSpec, error) {
	out := req

	url, err := rr.ResolveString(req.URL)
	if err != nil {
		return RequestSpec{}, wrapField(err, "request.url")
	}
	out.URL = url

	if req.Headers != nil {
		h, err := rr.ResolveHeaders(req.Headers)
		if err != nil {
			return RequestSpec{}, wrapField(err, "request.headers")
		}
		out.Headers = h
	} else {
		out.Headers = Headers{}
	}

	body, err := rr.ResolveBodySpec(req.Body)
	if err != nil {
		return RequestSpec{}, wrapField(err, "request.body")
	}
	out.Body = body

	return out, nil
}

// ResolveJSONValue recursively resolves string values inside JSON-like structures.
// Supported types: map[string]any, []any, string, numbers/bools/nil (left unchanged).
func (rr *RuntimeResolver) ResolveJSONValue(v any) (any, error) {
	switch t := v.(type) {
	case string:
		return rr.ResolveString(t)

	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			rv, err := rr.ResolveJSONValue(vv)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil

	case []any:
		out := make([]any, 0, len(t))
		for _, it := range t {
			rv, err := rr.ResolveJSONValue(it)
			if err != nil {
				return nil, err
			}
			out = append(out, rv)
		}
		return out, nil

	default:
		// numbers, bools, nil, etc.
		return v, nil
	}
}

func (r *VarResolver) resolveStringWith(vars Vars, builtins Vars, s string) (string, error) {
	// Fast path: no token start.
	if !strings.Contains(s, "{{") {
		return s, nil
	}

	var b strings.Builder
	b.Grow(len(s) + 16)

	for i := 0; i < len(s); {
		// Look for "{{"
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			start := i + 2

			// Find "}}"
			end := strings.Index(s[start:], "}}")
			if end < 0 {
				return "", &OpError{
					Op:   "vars.resolve",
					Kind: KindInvalidConfig,
					Err:  errors.New("unclosed placeholder"),
				}
			}
			end = start + end

			name := strings.TrimSpace(s[start:end])
			if name == "" {
				return "", &OpError{
					Op:   "vars.resolve",
					Kind: KindInvalidConfig,
					Err:  errors.New("empty placeholder"),
				}
			}

			val, ok := builtins[name]
			if !ok {
				val, ok = vars[name]
			}
			if !ok {
				return "", &OpError{
					Op:   "vars.resolve",
					Kind: KindMissingVar,
					Err:  fmt.Errorf("missing variable: %s", name),
				}
			}

			b.WriteString(val)
			i = end + 2
			continue
		}

		b.WriteByte(s[i])
		i++
	}

	return b.String(), nil
}

func wrapField(err error, field string) error {
	// Keep Kind information, but add context about which field was being resolved.
	return &OpError{
		Op:   "vars.resolve",
		Kind: kindFrom(err),
		Err:  fmt.Errorf("%s: %w", field, err),
	}
}

func kindFrom(err error) ErrorKind {
	var oe *OpError
	if errors.As(err, &oe) {
		return oe.Kind
	}
	return KindExecution
}

// uuidV4 generates a RFC4122-ish UUID v4 without external dependencies.
func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	// Version 4 (random)
	b[6] = (b[6] & 0x0f) | 0x40
	// Variant 10xxxxxx
	b[8] = (b[8] & 0x3f) | 0x80

	hexed := make([]byte, 36)
	hex.Encode(hexed[0:8], b[0:4])
	hexed[8] = '-'
	hex.Encode(hexed[9:13], b[4:6])
	hexed[13] = '-'
	hex.Encode(hexed[14:18], b[6:8])
	hexed[18] = '-'
	hex.Encode(hexed[19:23], b[8:10])
	hexed[23] = '-'
	hex.Encode(hexed[24:36], b[10:16])

	return string(hexed), nil
}
