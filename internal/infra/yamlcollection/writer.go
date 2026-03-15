package yamlcollection

import (
	"github.com/aalvaropc/lynix/internal/domain"
	"gopkg.in/yaml.v3"
)

type writeCollection struct {
	SchemaVersion int               `yaml:"schema_version"`
	Name          string            `yaml:"name"`
	Vars          map[string]string `yaml:"vars,omitempty"`
	Requests      []writeRequest    `yaml:"requests"`
}

type writeRequest struct {
	Name      string            `yaml:"name"`
	Method    string            `yaml:"method"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	JSON      map[string]any    `yaml:"json,omitempty"`
	Form      map[string]string `yaml:"form,omitempty"`
	Raw       string            `yaml:"raw,omitempty"`
	DelayMS   *int              `yaml:"delay_ms,omitempty"`
	TimeoutMS *int              `yaml:"timeout_ms,omitempty"`
	Tags      []string          `yaml:"tags,omitempty"`
	Assert    *writeAssertions  `yaml:"assert,omitempty"`
	Extract   map[string]string `yaml:"extract,omitempty"`
}

type writeAssertions struct {
	Status  *int                             `yaml:"status,omitempty"`
	Headers map[string]yamlJSONPathAssertion `yaml:"headers,omitempty"`
}

// MarshalCollection serializes a domain.Collection into YAML bytes.
func MarshalCollection(col domain.Collection) ([]byte, error) {
	wc := writeCollection{
		SchemaVersion: 1,
		Name:          col.Name,
		Vars:          map[string]string(col.Vars),
		Requests:      make([]writeRequest, 0, len(col.Requests)),
	}
	if len(wc.Vars) == 0 {
		wc.Vars = nil
	}

	for _, r := range col.Requests {
		wr := writeRequest{
			Name:    r.Name,
			Method:  string(r.Method),
			URL:     r.URL,
			Headers: map[string]string(r.Headers),
		}
		if len(wr.Headers) == 0 {
			wr.Headers = nil
		}

		switch r.Body.Type {
		case domain.BodyJSON:
			wr.JSON = r.Body.JSON
		case domain.BodyForm:
			wr.Form = r.Body.Form
		case domain.BodyRaw:
			wr.Raw = r.Body.Raw
		}

		wr.DelayMS = r.DelayMS
		wr.TimeoutMS = r.TimeoutMS

		if len(r.Tags) > 0 {
			wr.Tags = r.Tags
		}

		if r.Assert.Status != nil || len(r.Assert.Headers) > 0 {
			wa := &writeAssertions{Status: r.Assert.Status}
			if len(r.Assert.Headers) > 0 {
				wa.Headers = make(map[string]yamlJSONPathAssertion, len(r.Assert.Headers))
				for k, v := range r.Assert.Headers {
					wa.Headers[k] = yamlJSONPathAssertion{
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
			}
			wr.Assert = wa
		}

		if len(r.Extract) > 0 {
			wr.Extract = map[string]string(r.Extract)
		}

		wc.Requests = append(wc.Requests, wr)
	}

	return yaml.Marshal(wc)
}
