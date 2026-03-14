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
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
	JSON    map[string]any    `yaml:"json,omitempty"`
	Form    map[string]string `yaml:"form,omitempty"`
	Raw     string            `yaml:"raw,omitempty"`
	Tags    []string          `yaml:"tags,omitempty"`
	Assert  *writeAssertions  `yaml:"assert,omitempty"`
	Extract map[string]string `yaml:"extract,omitempty"`
}

type writeAssertions struct {
	Status *int `yaml:"status,omitempty"`
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

		if len(r.Tags) > 0 {
			wr.Tags = r.Tags
		}

		if r.Assert.Status != nil {
			wr.Assert = &writeAssertions{Status: r.Assert.Status}
		}

		if len(r.Extract) > 0 {
			wr.Extract = map[string]string(r.Extract)
		}

		wc.Requests = append(wc.Requests, wr)
	}

	return yaml.Marshal(wc)
}
