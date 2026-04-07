# Collection Format

Collections are YAML files stored in `collections/`. They describe a sequence of HTTP requests with optional assertions and variable extraction.

```yaml
schema_version: 1
name: Auth Flow

# Default variables (lowest priority — overridden by env vars)
vars:
  base_url: "https://api.example.com"
  timeout_ms: 2000

requests:
  - name: health-check
    method: GET
    url: "{{base_url}}/health"
    assert:
      status: 200
      max_ms: 500

  - name: login
    method: POST
    url: "{{base_url}}/auth/login"
    headers:
      Content-Type: "application/json"
    json:
      username: "{{username}}"
      password: "{{password}}"
    assert:
      status: 200
      max_ms: "{{timeout_ms}}"
      jsonpath:
        "$.token":
          exists: true
          matches: "^[A-Za-z0-9._-]+$"
    extract:
      auth_token: "$.token"          # available in all subsequent requests

  - name: list-users
    method: GET
    url: "{{base_url}}/v1/users"
    headers:
      Authorization: "Bearer {{auth_token}}"
    assert:
      status: 200
      jsonpath:
        "$.data":
          exists: true
        "$.data[0].active":
          eq: "true"
        "$.total":
          gt: 0
```

---

## Request Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for the request |
| `method` | Yes | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS` |
| `url` | Yes | URL — supports `{{variable}}` templating |
| `headers` | | Key-value map of HTTP headers (templating supported) |
| `json` | | JSON request body (object or array) |
| `form` | | Form URL-encoded body (string key-value map) |
| `raw` | | Raw text body |
| `tags` | | List of tags for selective execution with `--tags` |
| `delay_ms` | | Delay in milliseconds before executing this request |
| `timeout_ms` | | Per-request timeout in ms — aborts the request if exceeded (distinct from `max_ms` which is an assertion) |
| `assert` | | Assertions on the response |
| `extract` | | Variables to extract from the response body |

> Only one of `json`, `form`, or `raw` may be specified per request.

---

## Templating

Variables are injected using `{{variable_name}}` syntax. Works in URLs, headers, body values, and assertion values.

### Built-in Variables

Generated fresh per request:

| Variable | Value |
|----------|-------|
| `{{$uuid}}` | Random UUID v4 |
| `{{$timestamp}}` | Current Unix timestamp (seconds) |
| `{{$isoTimestamp}}` | ISO 8601 UTC timestamp (`2024-06-01T12:00:00Z`) |
| `{{$randomInt}}` | Random integer 0-9999 |
| `{{$randomString}}` | Random 8-character alphanumeric string |
| `{{$randomEmail}}` | Random email (`user_abc123@test.lynix`) |
| `{{$randomBool}}` | Random `true` or `false` |

---

## Assertions

Assertions are evaluated on every response regardless of previous failures. Each produces a named result with a pass/fail status.

### Status Code

```yaml
assert:
  status: 200
```

Checks the HTTP status code exactly.

### Latency

```yaml
assert:
  max_ms: 500          # fixed value
  max_ms: "{{timeout_ms}}"  # or from a variable
```

Checks that the response latency is at or below the threshold.

### JSONPath Assertions

Keyed by a JSONPath expression. Each key can combine multiple operators against the same path.

```yaml
assert:
  jsonpath:
    "$.data.field":
      exists: true          # path exists and value is non-empty
      eq: "expected"        # string equality
      contains: "partial"   # substring match
      matches: "^regex$"    # Go stdlib regexp
      gt: 10                # numeric greater-than
      lt: 1000              # numeric less-than
      not_eq: "forbidden"   # string inequality
      not_contains: "error" # substring absence
```

| Operator | Type | Description |
|----------|------|-------------|
| `exists` | bool | Path resolves to a non-null, non-empty value |
| `eq` | string | Value equals the given string (after type coercion) |
| `contains` | string | Value contains the given substring |
| `matches` | string | Value matches the given regular expression |
| `gt` | number | Numeric value is greater than threshold |
| `lt` | number | Numeric value is less than threshold |
| `not_eq` | string | Value does NOT equal the given string |
| `not_contains` | string | Value does NOT contain the given substring |

**JSONPath syntax** follows [PaesslerAG/jsonpath](https://github.com/PaesslerAG/jsonpath):
- `$.field` — top-level field
- `$.nested.field` — nested field
- `$.array[0].id` — array element
- `$.array[*].status` — all elements (returns array)

**Example:**
```yaml
assert:
  jsonpath:
    "$.token":
      exists: true
    "$.user_id":
      matches: "^user-[0-9]+$"
    "$.count":
      gt: 0
      lt: 100
```

### JSON Schema Validation

Validate the entire response body against a JSON Schema document. Supports Draft 7 and 2020-12.

**File reference** (path relative to the collection file):

```yaml
assert:
  schema: "schemas/user.json"
```

**Inline schema** (embedded in the collection YAML):

```yaml
assert:
  schema_inline:
    type: object
    required: ["id", "name"]
    properties:
      id:
        type: integer
      name:
        type: string
      email:
        type: string
        format: email
```

| Field | Type | Description |
|-------|------|-------------|
| `schema` | string | Path to a `.json` schema file, relative to the collection directory |
| `schema_inline` | object | Inline JSON Schema definition |

> `schema` and `schema_inline` are mutually exclusive — using both on the same request is a validation error.

On failure, the assertion message includes the instance location path and a description of the violation (e.g., `/user: missing property 'id'`).

**Combining with other assertions:**

```yaml
assert:
  status: 200
  max_ms: 1000
  schema: "schemas/user.json"
  jsonpath:
    "$.email":
      exists: true
```

Schema validation runs alongside status, latency, and JSONPath assertions — all results are reported independently.

---

## Variable Extraction

Extract values from a JSON response body and inject them into all subsequent requests in the collection.

```yaml
extract:
  auth_token: "$.token"
  user_id: "$.data.users[0].id"
  roles: "$.data.users[0].roles"   # array -> stored as JSON string
```

Extraction rules are applied in sorted order. If a rule fails (path not found, empty value), the error is reported but remaining extractions continue.

### Value Conversion Rules

| Response value | Stored as |
|---------------|-----------|
| String | String |
| Number / bool | String representation |
| Single-element array | The element's value |
| Multi-element array | JSON string |
| Object/map | JSON string |
| Null or empty | Extraction fails |

Extracted variables are available in all subsequent requests within the same run.

---

## Variable Resolution Order

When the same variable name appears in multiple places, the highest priority wins:

```
secrets.local.yaml  >  environment YAML  >  collection vars  >  built-ins
```

| Source | Priority | Description |
|--------|----------|-------------|
| `secrets.local.yaml` | Highest | Local overrides — gitignored |
| Environment YAML | High | Selected environment file (`-e dev`) |
| Collection `vars` | Medium | Defined in the collection itself |
| Built-ins (`$uuid`, `$timestamp`) | Lowest | Generated at request time |

Variables extracted from responses are merged into the running set and available to the next request.
