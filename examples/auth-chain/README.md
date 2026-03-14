# Auth Chain Example

Simulates an authentication flow: login, use extracted token to fetch profile, then fetch user data.

## Run

```bash
lynix run -c examples/auth-chain/collection.yaml -e examples/auth-chain/env.yaml --no-save
```

Run only auth-tagged requests:

```bash
lynix run -c examples/auth-chain/collection.yaml -e examples/auth-chain/env.yaml --tags auth --no-save
```

## What it demonstrates

- Variable extraction from login response (`auth_token`)
- Token injection into subsequent requests (headers + URL)
- Environment variables for credentials (`username`, `password`)
- Tag-based selective execution (`--tags auth`)
- Request chaining: login -> profile -> data
