# Error Handling

## Request Errors

If a request cannot be completed (network failure, timeout, DNS error), the result includes a structured error:

| Kind | Description |
|------|-------------|
| `dns` | DNS resolution failed |
| `connection` | Could not connect to host |
| `timeout` | Request exceeded HTTP client timeout |
| `canceled` | Run was canceled by the user |
| `http` | HTTP protocol error |
| `unknown` | Unexpected error |

---

## Missing Variables

If a template placeholder like `{{my_var}}` cannot be resolved, the request fails with a `missing_variable` error pointing to the exact variable name. The CLI shows a human-readable message identifying which variable is missing and in which request it was referenced.
