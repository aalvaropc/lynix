package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// BuildRequest builds an HTTP request from a domain RequestSpec.
func BuildRequest(ctx context.Context, spec domain.RequestSpec) (*http.Request, error) {
	if strings.TrimSpace(spec.URL) == "" {
		return nil, &domain.OpError{
			Op:   "httpclient.build",
			Kind: domain.KindInvalidConfig,
			Err:  domain.ErrInvalidRequest,
		}
	}

	var bodyReader *bytes.Reader
	contentType := ""

	switch spec.Body.Type {
	case domain.BodyNone:
		bodyReader = bytes.NewReader(nil)
	case domain.BodyJSON:
		if spec.Body.JSON != nil {
			payload, err := json.Marshal(spec.Body.JSON)
			if err != nil {
				return nil, &domain.OpError{
					Op:   "httpclient.build",
					Kind: domain.KindInvalidConfig,
					Err:  err,
				}
			}
			bodyReader = bytes.NewReader(payload)
			contentType = "application/json"
		} else {
			bodyReader = bytes.NewReader(nil)
		}
	case domain.BodyForm:
		if spec.Body.Form != nil {
			values := url.Values{}
			for k, v := range spec.Body.Form {
				values.Set(k, v)
			}
			bodyReader = bytes.NewReader([]byte(values.Encode()))
			contentType = "application/x-www-form-urlencoded"
		} else {
			bodyReader = bytes.NewReader(nil)
		}
	case domain.BodyRaw:
		if strings.TrimSpace(spec.Body.Raw) != "" {
			bodyReader = bytes.NewReader([]byte(spec.Body.Raw))
			contentType = spec.Body.ContentType
		} else {
			bodyReader = bytes.NewReader(nil)
		}
	default:
		return nil, &domain.OpError{
			Op:   "httpclient.build",
			Kind: domain.KindInvalidConfig,
			Err:  domain.ErrInvalidRequest,
		}
	}

	req, err := http.NewRequestWithContext(ctx, string(spec.Method), spec.URL, bodyReader)
	if err != nil {
		return nil, &domain.OpError{
			Op:   "httpclient.build",
			Kind: domain.KindInvalidConfig,
			Err:  err,
		}
	}

	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	return req, nil
}
