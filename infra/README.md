# Infra

Dockerfiles and deployment scripts.

## Dockerfiles

| File | Base | Description |
|---|---|---|
| `docker/manager/Dockerfile` | Multi-stage, scratch final | Production manager binary |
| `docker/manager/Dockerfile.dev` | Go builder with hot reload | Development with live recompilation |
| `docker/gateway/Dockerfile` | Multi-stage, scratch final | Production gateway binary |
| `docker/gateway/Dockerfile.dev` | Go builder with hot reload | Development with live recompilation |

All production images use `scratch` base with a non-root `USER` directive.

## Scripts

| Script | Description |
|---|---|
| `scripts/initdb.sql` | Postgres initialization — creates the `manager_user` role and database |
| `../scripts/pool-backup.sh` | Database backup helper (pg_dump to timestamped file) |

## Usage

The Dockerfiles are referenced by `docker-compose.yml` (production) and
`docker-compose.dev.yml` (development with hot reload):

```bash
# Production
docker compose up -d

# Development
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
# or: make dev
```
