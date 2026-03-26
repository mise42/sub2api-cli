# sub2api-cli

`sub2api-cli` is a standalone interactive CLI for importing OpenAI OAuth accounts into Sub2API.

It reuses the existing Sub2API admin endpoints:

- `POST /api/v1/admin/openai/generate-auth-url`
- `POST /api/v1/admin/openai/create-from-oauth`

Current version authenticates with `x-api-key`, so the supplier needs a valid Sub2API admin API key.

## Build

```bash
cd /Users/mise42/Work/untrusted/sub2api-suite/sub2api-cli
go build ./cmd/sub2api-cli
```

## Example

```bash
./sub2api-cli \
  --server https://sub2api.example.com \
  --api-key admin-xxxxxxxx \
  --group-id 12 \
  --group-id 18 \
  --concurrency 1
```

What it does:

1. Starts a local callback server on `http://localhost:1455/auth/callback`
2. Requests an OAuth URL from Sub2API
3. Opens the browser automatically
4. Waits for the OAuth callback locally
5. Calls `create-from-oauth`
6. Prints the created account as JSON

## Flags

- `--server`: Sub2API base URL. The CLI appends `/api/v1` automatically when needed.
- `--api-key`: Admin `x-api-key`. You can also use `SUB2API_ADMIN_API_KEY`.
- `--platform`: `openai` or `sora`
- `--name`: Optional account name override
- `--group-id`: Bind to one or more groups. Repeat the flag or pass comma-separated IDs.
- `--proxy-id`: Optional Sub2API proxy ID
- `--concurrency`: Account concurrency, defaults to `1`
- `--priority`: Account priority, defaults to `0`
- `--redirect-url`: Local callback URL, defaults to `http://localhost:1455/auth/callback`
- `--no-open`: Do not auto-open the browser

## Environment Variables

- `SUB2API_SERVER`
- `SUB2API_BASE_URL`
- `SUB2API_ADMIN_API_KEY`

## Notes

- The callback server is local only and exits after the flow completes.
- If the browser cannot be opened automatically, the CLI prints the OAuth URL for manual opening.
- This project is intentionally standalone and does not import code from the main `sub2api` repository.
