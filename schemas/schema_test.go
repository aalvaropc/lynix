package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func schemasDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func TestSchemaFiles_AreValidJSONSchema(t *testing.T) {
	dir := schemasDir()
	files := []string{
		"collection.schema.json",
		"environment.schema.json",
		"workspace.schema.json",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read schema file: %v", err)
			}

			var doc any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatalf("unmarshal schema: %v", err)
			}

			c := jsonschema.NewCompiler()
			if err := c.AddResource(name, doc); err != nil {
				t.Fatalf("add resource: %v", err)
			}

			_, err = c.Compile(name)
			if err != nil {
				t.Fatalf("compile schema %s: %v", name, err)
			}
		})
	}
}
