# Editor Integration

## VS Code / YAML Schema Validation

Lynix ships JSON Schema files under `schemas/`. With the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml), add to `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "./schemas/collection.schema.json": "collections/*.yaml",
    "./schemas/environment.schema.json": "env/*.yaml",
    "./schemas/workspace.schema.json": "lynix.yaml"
  }
}
```

This gives auto-complete and inline validation for collection, environment, and workspace YAML files.
