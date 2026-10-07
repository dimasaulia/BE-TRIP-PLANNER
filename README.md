# Boilerplate Golang

Multipurpose Go boilerplate for Open Suite services.

## Run

```bash
go run ./cmd/api
```

## Local Dependencies

```bash
docker compose up -d
make migrate-up
make dev
```

PostgreSQL and Redis configuration live in `.env`. Use `.env.example` as the tracked template.

## Development With Air

```bash
go install github.com/air-verse/air@latest
air
```

## Health Check

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```
