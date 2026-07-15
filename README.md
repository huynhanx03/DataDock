# DataDock

DataDock is a self-hosted database workbench for PostgreSQL, MySQL, and MariaDB. It provides connection management, catalog browsing, typed table data, SQL execution and history, staged mutations, transactions, schema changes, saved queries, and operational views from one compact web interface.

DataDock stores its own metadata in a local SQLite volume. Remote database credentials are encrypted with a key that stays in your `.env` file.

## Start with one command

Docker Engine and the Docker Compose plugin are the only runtime prerequisites.

```bash
make up
```

The command creates `.env` when absent, generates a random base64-encoded 32-byte encryption key, builds both images, starts the stack, and waits for both health checks. Open [http://localhost:8088](http://localhost:8088).

The default deployment uses the real API. For a visual-only demo with local mock data:

```bash
DATADOCK_WEB_DATA_SOURCE=mock make up
```

Changing the frontend data source or API base URL requires rebuilding the web image.

## Supported database engines

| Engine | Status | Default port |
| --- | --- | ---: |
| PostgreSQL | Supported | 5432 |
| MySQL | Supported | 3306 |
| MariaDB | Supported | 3306 |
| SQLite, SQL Server, Oracle, ClickHouse, Redis, MongoDB | Planned | — |

When DataDock runs in Docker, a database on the Docker host is reached as `host.docker.internal`, not `localhost`.

## Make commands

| Command | Purpose |
| --- | --- |
| `make setup` | Create `.env` and generate its encryption key when missing |
| `make up` | Build, start, and health-check the complete stack |
| `make ps` | Show container and health state |
| `make logs` | Follow API and web logs |
| `make down` | Stop containers while preserving metadata |
| `make restart` | Rebuild and recreate the stack |
| `make check` | Validate Compose, service tests, Go checks, and website builds |
| `make clean` | Stop containers while preserving metadata |
| `make purge-data CONFIRM=delete` | Permanently remove the metadata volume |
| `make api` | Run the API locally on port 8080 |
| `make web` | Run Vite locally on port 5173 against the local API |

## Configuration

The generated `.env` is the deployment configuration. Common settings are:

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATADOCK_WEB_BIND_ADDRESS` | `127.0.0.1` | Host interface that exposes the web gateway |
| `DATADOCK_WEB_PORT` | `8088` | Host port for the web gateway |
| `DATADOCK_WEB_DATA_SOURCE` | `api` | Frontend adapter, `api` or explicit `mock` |
| `DATADOCK_WEB_API_BASE_URL` | empty | External API origin; empty uses the same-origin Nginx proxy |
| `DATADOCK_ENCRYPTION_KEY` | generated | Base64-encoded 32-byte key for saved secrets |
| `DATADOCK_CORS_ORIGINS` | `http://localhost:8088` | Comma-separated browser origins allowed by the API |
| `DATADOCK_LOG_LEVEL` | `info` | Structured API log level |
| `DATADOCK_POOL_MAX_OPEN` | `10` | Maximum open remote connections per pool |
| `DATADOCK_QUERY_MAX_ROWS` | `1000` | Maximum rows returned by a query execution |
| `DATADOCK_QUERY_MAX_BYTES` | `10485760` | Maximum encoded query-result bytes |
| `DATADOCK_QUERY_MAX_CONCURRENCY` | `8` | Maximum concurrent query executions |
| `DATADOCK_TRANSACTION_TTL` | `15m` | Idle lifetime of an explicit transaction |

See [Self-hosting](docs/self-hosting.md) for every setting, reverse-proxy guidance, database connectivity, backup, restore, upgrades, and troubleshooting. See [Backend API](docs/backend-api.md) for the complete HTTP contract.

## Local development

Run these in separate terminals:

```bash
make api
make web
```

`make api` loads the root `.env`. `make web` selects API mode and points Vite to `http://localhost:8080` unless `VITE_DATA_SOURCE` or `VITE_API_BASE_URL` is explicitly supplied.

## Security boundary

DataDock currently has no application login or authorization layer. Keep it on a trusted machine or network, behind a VPN, or behind an authenticated reverse proxy. The default Compose configuration binds only to loopback and does not publish the API container directly.

Never lose or casually replace `DATADOCK_ENCRYPTION_KEY`. A metadata backup without the matching key cannot decrypt saved database, proxy, or SSH credentials.