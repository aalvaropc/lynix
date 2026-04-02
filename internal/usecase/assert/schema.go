package assert

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// SchemaValidate validates a response body against a JSON Schema.
// schemaBytes must be a valid JSON Schema document.
// truncated indicates the body was cut off and may not be valid JSON.
func SchemaValidate(schemaBytes []byte, body []byte, truncated bool) domain.AssertionResult {
	if len(schemaBytes) == 0 {
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: "schema is empty",
		}
	}

	if len(body) == 0 {
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: "cannot validate schema: response has no body",
		}
	}

	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: fmt.Sprintf("invalid schema JSON: %v", err),
		}
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaDoc); err != nil {
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: fmt.Sprintf("failed to add schema resource: %v", err),
		}
	}

	sch, err := c.Compile("schema.json")
	if err != nil {
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: fmt.Sprintf("failed to compile schema: %v", err),
		}
	}

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		msg := fmt.Sprintf("response body is not valid JSON: %v", err)
		if truncated {
			msg = fmt.Sprintf("response body was truncated (>256KB) and is not valid JSON: %v", err)
		}
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: msg,
		}
	}

	if err := sch.Validate(doc); err != nil {
		msg := formatSchemaError(err)
		return domain.AssertionResult{
			Name:    "schema",
			Passed:  false,
			Message: msg,
		}
	}

	return domain.AssertionResult{
		Name:    "schema",
		Passed:  true,
		Message: "response body matches schema",
	}
}

func formatSchemaError(err error) string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return fmt.Sprintf("schema validation failed: %v", err)
	}

	var msgs []string
	collectLeafErrors(ve, &msgs)
	if len(msgs) == 0 {
		return fmt.Sprintf("schema validation failed: %v", err)
	}
	if len(msgs) == 1 {
		return fmt.Sprintf("schema validation failed: %s", msgs[0])
	}
	return fmt.Sprintf("schema validation failed (%d errors): %s", len(msgs), strings.Join(msgs, "; "))
}

var schemaPrinter = message.NewPrinter(language.English)

func collectLeafErrors(ve *jsonschema.ValidationError, msgs *[]string) {
	if len(ve.Causes) == 0 {
		loc := "/" + strings.Join(ve.InstanceLocation, "/")
		desc := ve.ErrorKind.LocalizedString(schemaPrinter)
		*msgs = append(*msgs, fmt.Sprintf("%s: %s", loc, desc))
		return
	}
	for _, cause := range ve.Causes {
		collectLeafErrors(cause, msgs)
	}
}
