# Trip Planner Backend

Go + PostgreSQL backend for the collaborative trip timeline planner (REST, WebSocket, Google Calendar sync worker), implementing the TRD "Backend – Trip Timeline Planner". Built on the Open Suite Go boilerplate (modules of `controllers/services/repositories/dto`, Wire, goose SQL migrations).

## Run

```bash
cp .env.example .env            # set TOKEN_ENC_KEY (openssl rand -hex 32), DEV_LOGIN_ENABLED=true for local testing
docker compose up -d
go run github.com/pressly/goose/v3/cmd/goose -dir migrations postgres "$DATABASE_URL" up
go run ./cmd/api                # or: air
```

Health: `curl localhost:8080/health/live`. If port 8080 is taken, change `APP_PORT` (and `serverUrl` in the Bruno environment).

## Layout

| Path | Purpose |
| --- | --- |
| `internal/modules/{auth,trips,invites,library,schedule,uploads,realtime,calendar}` | Feature modules |
| `internal/platform/{google,realtime,storage,crypto}` | Google OAuth/Calendar client, WebSocket hub, image storage, token crypto |
| `internal/shared/{access,apperror,attachments,syncqueue,...}` | Role checks, uniform errors, shared helpers |
| `migrations/` | goose migrations (users, trips, library, schedule with exclusion constraint, calendar sync) |
| `../bruno/trip-planner/` | Bruno API collection (monorepo root) |

After adding providers, regenerate Wire: `go run github.com/google/wire/cmd/wire ./internal/app`.

## API conventions

- Base path `/api/v1`. Envelope: `{success, message, data}`; failures add `error: {code, message, details}`. Messages follow `Accept-Language` (`id` default, `en`).
- Statuses: 400 validation (incl. non-UTC time `invalid_time`), 401, 403 role too low, 404 not found *or not a member*, 409 `version_conflict` / `library_item_in_use`, 413, 422 `outside_trip_range` / `block_overlap` / `last_owner`, 429.
- All times are UTC RFC 3339 with `Z`; trip dates are `YYYY-MM-DD` in the trip timezone. The frontend converts.
- Library items carry the workspace design fields `category` (free text, max 40, colour chosen per browser), `default_duration_min` (30–240, step 5, default 60) and `banner_url` (same rule as the trip banner); blocks include `library_item.category` and their own `hue` (0–359 or null) to colour free-text notes.
- `PATCH` of library items and blocks needs `version`; a stale one returns 409 with `error.details.current`.
- Mutations accept `X-Client-Id`, echoed as `origin` in the resulting WebSocket events.
- Mutations check the `Origin` header against `ALLOWED_ORIGINS` when one is present (browsers always send it; non-browser clients such as Bruno send none).

## WebSocket

`GET /api/v1/trips/{id}/ws` (session cookie, allowed `Origin`). Envelope `{"v":1,"type","seq","origin","payload"}`.
Server → client: `hello`, `presence.update`, `library.created|updated|deleted`, `block.created|updated|deleted`, `block.preview`, `block.preview_end`, `trip.updated`, `member.changed`, `error`.
Client → server: `presence.set` (`{"viewing":{"type":"library_item","id":"…"}}` or `{"type":"schedule"}` or `null`), `block.preview`, `block.preview_end` (owner/editor only, 20/s).
`seq` increases once per committed event; previews and presence reuse the current value. On a gap or reconnect, refetch via REST. Try it: `npx wscat -H "Cookie: session=<value>" -H "Origin: http://localhost:3000" -c ws://localhost:8080/api/v1/trips/<id>/ws`.

## Google Cloud setup

1. Create an OAuth client (type *Web application*); add redirect URIs `GOOGLE_REDIRECT_URL` and `GOOGLE_CALENDAR_REDIRECT_URL`.
2. Enable the Google Calendar API; add scopes `openid email profile` and `.../auth/calendar.app.created` (confirmed in Google's scope list).
3. While the project is in *Testing*, only listed test users can connect Calendar and refresh tokens may expire after 7 days; verification is needed before going public.
4. Set `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `TOKEN_ENC_KEY`.

Login without a frontend: open `/api/v1/auth/google/login?next=/api/v1/me` in a browser; it lands on your profile JSON and the session cookie is set. With `APP_BASE_URL` pointing at the API this works as is. Calendar: `/api/v1/calendar/connect`, then `PUT /trips/{id}/calendar-sync`.

## Bruno

Open `bruno/trip-planner` (monorepo root) in Bruno (*Open Collection*), choose the **Local** environment, run **01 Auth / Dev Login - User A**. The folder **99 Smoke Flow** is an ordered end-to-end run (33 requests, 57 assertions). For real Google sessions paste the browser cookie into `sessionCookie`. CLI: `npx @usebruno/cli run "99 Smoke Flow" --env Local` (inside the collection folder).

## Tests

```bash
go test ./...                                              # unit tests
TEST_DATABASE_URL=postgres://… go test -race ./...         # + integration (WIPES the public schema)
```

Integration tests cover roles, invites, version conflicts, overlap (including parallel requests), library export, uploads, WebSocket presence/broadcast/previews, and the Calendar worker against a fake Google.

## Notes

- `DEV_LOGIN_ENABLED` adds a password-less login for local testing only. Never enable it in production.
- Errors keep the boilerplate envelope and add the TRD's `error` object, so success bodies are wrapped in `data`.
- Not included from TRD milestone 7: metrics, backup, load test, OpenAPI document.
