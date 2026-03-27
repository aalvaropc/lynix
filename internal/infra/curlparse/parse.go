package curlparse

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// Result holds the parsed collection and any warnings about unsupported flags.
type Result struct {
	Collection domain.Collection
	Warnings   []string
	Insecure   bool // curl -k/--insecure was present
}

// Parse converts a curl command string into a domain.Collection.
func Parse(input string) (Result, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return Result{}, fmt.Errorf("tokenize: %w", err)
	}
	if len(tokens) == 0 {
		return Result{}, fmt.Errorf("empty curl command")
	}

	// Strip leading "curl" (or path ending in /curl).
	first := tokens[0]
	if first == "curl" || strings.HasSuffix(first, "/curl") {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return Result{}, fmt.Errorf("curl command has no arguments")
	}

	var (
		method   string
		rawURL   string
		headers  = map[string]string{}
		bodyData string
		hasBody  bool
		jsonFlag bool
		warnings []string
	)

	var insecureFlag, locationFlag bool

	// Flags that are known but unsupported — consume their value if needed.
	unsupportedNoArg := map[string]bool{
		"--compressed": true,
		"-v":           true, "--verbose": true,
		"-s": true, "--silent": true,
		"-S": true, "--show-error": true,
		"-i": true, "--include": true,
		"-g": true, "--globoff": true,
	}
	unsupportedWithArg := map[string]bool{
		"--cert": true, "--key": true,
		"-o": true, "--output": true,
	}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// -X / --request
		if tok == "-X" || tok == "--request" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag %s requires a value", tok)
			}
			method = strings.ToUpper(tokens[i])
			continue
		}

		// -H / --header
		if tok == "-H" || tok == "--header" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag %s requires a value", tok)
			}
			k, v, ok := parseHeader(tokens[i])
			if ok {
				headers[k] = v
			}
			continue
		}

		// -d / --data / --data-raw / --data-binary
		if tok == "-d" || tok == "--data" || tok == "--data-raw" || tok == "--data-binary" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag %s requires a value", tok)
			}
			val := tokens[i]
			if strings.HasPrefix(val, "@") {
				warnings = append(warnings, fmt.Sprintf("file reference %q in %s is not supported; data ignored", val, tok))
				continue
			}
			bodyData = val
			hasBody = true
			continue
		}

		// --json (implies Content-Type + Accept + POST)
		if tok == "--json" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag --json requires a value")
			}
			bodyData = tokens[i]
			hasBody = true
			jsonFlag = true
			continue
		}

		// -u / --user (basic auth)
		if tok == "-u" || tok == "--user" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag %s requires a value", tok)
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(tokens[i]))
			headers["Authorization"] = "Basic " + encoded
			continue
		}

		// -F / --form (multipart — unsupported)
		if tok == "-F" || tok == "--form" {
			i++
			if i >= len(tokens) {
				return Result{}, fmt.Errorf("flag %s requires a value", tok)
			}
			warnings = append(warnings, fmt.Sprintf("multipart form data (%s) is not supported; field %q ignored", tok, tokens[i]))
			continue
		}

		// -k / --insecure (skip TLS verification)
		if tok == "-k" || tok == "--insecure" {
			insecureFlag = true
			continue
		}

		// -L / --location (follow redirects)
		if tok == "-L" || tok == "--location" {
			locationFlag = true
			continue
		}

		// Known unsupported flags without arguments.
		if unsupportedNoArg[tok] {
			warnings = append(warnings, fmt.Sprintf("flag %s is not supported and was ignored", tok))
			continue
		}

		// Known unsupported flags with arguments.
		if unsupportedWithArg[tok] {
			warnings = append(warnings, fmt.Sprintf("flag %s is not supported and was ignored", tok))
			i++ // skip the value
			continue
		}

		// Unknown flag — warn but continue.
		if strings.HasPrefix(tok, "-") {
			warnings = append(warnings, fmt.Sprintf("unknown flag %q was ignored", tok))
			continue
		}

		// Bare token → URL.
		if rawURL == "" {
			rawURL = tok
		}
	}

	if rawURL == "" {
		return Result{}, fmt.Errorf("no URL found in curl command")
	}

	// Method inference.
	if method == "" {
		if hasBody {
			method = "POST"
		} else {
			method = "GET"
		}
	}

	// JSON flag implies headers.
	if jsonFlag {
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
		if _, ok := headers["Accept"]; !ok {
			headers["Accept"] = "application/json"
		}
	}

	// Extract base_url from URL.
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	path := parsedURL.RequestURI()
	reqURL := "{{base_url}}" + path

	// Build body.
	var body domain.BodySpec
	if hasBody && bodyData != "" {
		var jsonBody any
		if err := json.Unmarshal([]byte(bodyData), &jsonBody); err == nil {
			switch jsonBody.(type) {
			case map[string]any, []any:
				body = domain.BodySpec{Type: domain.BodyJSON, JSON: jsonBody}
			default:
				body = domain.BodySpec{Type: domain.BodyRaw, Raw: bodyData}
			}
		} else {
			body = domain.BodySpec{Type: domain.BodyRaw, Raw: bodyData}
		}
	} else {
		body = domain.BodySpec{Type: domain.BodyNone}
	}

	// Derive request name from method + path.
	reqName := deriveRequestName(method, parsedURL.Path)

	// Derive collection name from host.
	colName := "Imported from " + parsedURL.Host

	req := domain.RequestSpec{
		Name:    reqName,
		Method:  domain.HTTPMethod(method),
		URL:     reqURL,
		Headers: domain.Headers(headers),
		Body:    body,
	}
	if locationFlag {
		t := true
		req.FollowRedirects = &t
	}

	col := domain.Collection{
		SchemaVersion: 1,
		Name:          colName,
		Vars:          domain.Vars{"base_url": baseURL},
		Requests:      []domain.RequestSpec{req},
	}

	return Result{Collection: col, Warnings: warnings, Insecure: insecureFlag}, nil
}

// tokenize splits a shell command into tokens, handling quoting and backslash escapes.
func tokenize(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if escaped {
			if c == '\n' {
				// Backslash-newline = line continuation, skip both.
				escaped = false
				continue
			}
			current.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingle {
			escaped = true
			continue
		}

		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (c == ' ' || c == '\t' || c == '\n' || c == '\r') && !inSingle && !inDouble {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in input")
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// parseHeader splits "Key: Value" into (key, value, true).
func parseHeader(h string) (string, string, bool) {
	idx := strings.IndexByte(h, ':')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(h[:idx]), strings.TrimSpace(h[idx+1:]), true
}

// deriveRequestName creates a name like "post-v1-users" from method + path.
func deriveRequestName(method, path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return strings.ToLower(method)
	}
	// Replace slashes with hyphens, limit to reasonable length.
	slug := strings.ReplaceAll(path, "/", "-")
	slug = strings.ToLower(slug)
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return strings.ToLower(method) + "-" + slug
}
