# Importing

Lynix can import existing API definitions from curl commands and Postman collections, converting them to Lynix YAML format.

---

## Import from curl

```bash
lynix import curl 'curl -X POST -H "Content-Type: application/json" -d '\''{"name":"test"}'\'' https://api.example.com/users'
lynix import curl 'curl https://api.example.com/health' -o collections/health.yaml
lynix import curl --from-file saved-curl.txt --name "My API"
```

### Supported curl Flags

`-X`, `-H`, `-d`/`--data`/`--data-raw`, `--json`, `-u` (basic auth).

### Unsupported curl Flags (warned)

`--compressed`, `-k`, `-L`, `--cert`, `--key`, `-o`, `-v`, `-s`, `-F` (multipart), `-d @file`.

### Base URL Extraction

The importer extracts `base_url` as a variable and rewrites the URL to use `{{base_url}}`.

---

## Import from Postman

```bash
lynix import postman collection.json
lynix import postman collection.json -o collections/imported.yaml
lynix import postman collection.json --name "Renamed API"
```

### Supported Postman Features

Requests with headers, JSON bodies (`raw` + `language: json`), URL-encoded bodies, collection variables, nested folders (flattened with dot-prefix names).

### Unsupported Postman Features (warned)

Pre-request/test scripts, auth blocks, multipart form-data, Postman dynamic variables (`{{$randomInt}}`).

### Variable Syntax

Postman `{{variable}}` syntax passes through directly as it matches Lynix templating.

### Folder Flattening

Nested Postman folders are flattened into a single list of requests. Folder names are used as dot-prefixed request names (e.g., `Users.Create User`).

---

## Migrate from Existing Tools

Already have curl commands or Postman collections? Import them in seconds:

```bash
# From a curl command (e.g., copied from browser DevTools)
lynix import curl 'curl -H "Authorization: Bearer tok" https://api.example.com/users' -o collections/users.yaml

# From a Postman export
lynix import postman my-collection.json -o collections/imported.yaml

# Then run immediately
lynix run -c imported -e dev
```
