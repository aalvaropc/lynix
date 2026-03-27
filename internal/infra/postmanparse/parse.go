package postmanparse

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// Result holds the parsed collection and any warnings about unsupported features.
type Result struct {
	Collection domain.Collection
	Warnings   []string
}

// Parse reads a Postman Collection v2.1 JSON from r and converts it to a domain.Collection.
func Parse(r io.Reader) (Result, error) {
	var pc PostmanCollection
	if err := json.NewDecoder(r).Decode(&pc); err != nil {
		return Result{}, fmt.Errorf("decode postman collection: %w", err)
	}

	var warnings []string

	// Collection-level events.
	for _, ev := range pc.Event {
		warnings = append(warnings, fmt.Sprintf("collection-level %q script was ignored", ev.Listen))
	}

	// Collection variables → vars.
	vars := domain.Vars{}
	for _, v := range pc.Variable {
		vars[v.Key] = v.Value
	}

	// Flatten items.
	var requests []domain.RequestSpec
	requests, warnings = flattenItems(pc.Item, "", requests, warnings)

	col := domain.Collection{
		SchemaVersion: 1,
		Name:          pc.Info.Name,
		Vars:          vars,
		Requests:      requests,
	}
	if len(col.Vars) == 0 {
		col.Vars = nil
	}

	return Result{Collection: col, Warnings: warnings}, nil
}

func flattenItems(items []PostmanItem, prefix string, reqs []domain.RequestSpec, warnings []string) ([]domain.RequestSpec, []string) {
	for _, item := range items {
		// Folder — recurse with prefix.
		if item.Request == nil && len(item.Item) > 0 {
			folderPrefix := item.Name
			if prefix != "" {
				folderPrefix = prefix + "." + item.Name
			}
			warnings = append(warnings, fmt.Sprintf("folder %q was flattened", item.Name))
			reqs, warnings = flattenItems(item.Item, folderPrefix, reqs, warnings)
			continue
		}

		if item.Request == nil {
			continue
		}

		// Item-level events.
		for _, ev := range item.Event {
			warnings = append(warnings, fmt.Sprintf("request %q: %q script was ignored", item.Name, ev.Listen))
		}

		req, ws := mapRequest(item, prefix)
		warnings = append(warnings, ws...)
		reqs = append(reqs, req)
	}
	return reqs, warnings
}

func mapRequest(item PostmanItem, prefix string) (domain.RequestSpec, []string) {
	var warnings []string
	pr := item.Request

	name := item.Name
	if prefix != "" {
		name = prefix + "." + item.Name
	}
	// Sanitize name for YAML friendliness.
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ToLower(name)

	// Method.
	method := strings.ToUpper(pr.Method)
	if method == "" {
		method = "GET"
	}

	// URL.
	rawURL := pr.URL.Raw

	// Headers.
	headers := domain.Headers{}
	for _, h := range pr.Header {
		headers[h.Key] = h.Value
	}

	// Auth — warn.
	if pr.Auth != nil {
		warnings = append(warnings, fmt.Sprintf("request %q: auth type %q was ignored", item.Name, pr.Auth.Type))
	}

	// Body.
	body := domain.BodySpec{Type: domain.BodyNone}
	if pr.Body != nil {
		switch pr.Body.Mode {
		case "raw":
			// Check if it's JSON.
			isJSON := false
			if pr.Body.Options != nil && pr.Body.Options.Raw.Language == "json" {
				isJSON = true
			}
			if isJSON {
				var jsonBody any
				if err := json.Unmarshal([]byte(pr.Body.Raw), &jsonBody); err == nil {
					switch jsonBody.(type) {
					case map[string]any, []any:
						body = domain.BodySpec{Type: domain.BodyJSON, JSON: jsonBody}
					default:
						body = domain.BodySpec{Type: domain.BodyRaw, Raw: pr.Body.Raw}
					}
				} else {
					body = domain.BodySpec{Type: domain.BodyRaw, Raw: pr.Body.Raw}
				}
			} else {
				body = domain.BodySpec{Type: domain.BodyRaw, Raw: pr.Body.Raw}
			}
		case "urlencoded":
			form := map[string]string{}
			for _, kv := range pr.Body.URLEncoded {
				form[kv.Key] = kv.Value
			}
			body = domain.BodySpec{Type: domain.BodyForm, Form: form}
		case "formdata":
			warnings = append(warnings, fmt.Sprintf("request %q: multipart form-data body is not supported", item.Name))
		case "":
			// no body
		default:
			warnings = append(warnings, fmt.Sprintf("request %q: body mode %q is not supported", item.Name, pr.Body.Mode))
		}
	}

	// Check for Postman dynamic variables in URL.
	if strings.Contains(rawURL, "{{$") {
		warnings = append(warnings, fmt.Sprintf("request %q: Postman dynamic variables (e.g. {{$randomInt}}) are not supported", item.Name))
	}

	req := domain.RequestSpec{
		Name:    name,
		Method:  domain.HTTPMethod(method),
		URL:     rawURL,
		Headers: headers,
		Body:    body,
	}

	return req, warnings
}
