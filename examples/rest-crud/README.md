# REST CRUD Example

Full Create-Read-Update-Delete lifecycle against a REST API.

## Run

```bash
lynix run -c examples/rest-crud/collection.yaml -e examples/rest-crud/env.yaml --no-save
```

## What it demonstrates

- POST with JSON body and `status: 201` assertion
- Variable extraction (`post_id` from create response)
- GET with JSON Schema inline validation
- PUT with JSONPath equality check
- DELETE with status assertion
- Request chaining across a full CRUD flow
