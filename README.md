# DataDock

DataDock is a self-hosted database workspace for PostgreSQL, MySQL and MariaDB. It keeps its application metadata locally while connecting to databases you choose.

## Quick start

Install Docker Engine with the Compose plugin, then prepare the environment and start the stack:

```bash
make setup
```

Generate a 32-byte encryption key and replace `DATADOCK_ENCRYPTION_KEY` in `.env`:

```bash
openssl rand -base64 32
```

Start DataDock:

```bash
make up
```

Open `http://localhost:8088`. The API health endpoint is available at `http://localhost:8088/healthz`.

## Configuration

Copy `.env.example` to `.env` with `make setup` before the first run.

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATADOCK_WEB_PORT` | `8088` | Host port for the web application |
| `DATADOCK_WEB_DATA_SOURCE` | `mock` | Frontend adapter: `mock` for the rich demo workspace or `api` for the live backend |
| `DATADOCK_WEB_API_BASE_URL` | empty | Optional external API base URL; an empty value uses the same-origin Nginx proxy |
| `DATADOCK_ENCRYPTION_KEY` | required | Base64-encoded 32-byte key used to encrypt saved database credentials |
| `DATADOCK_CORS_ORIGINS` | `http://localhost:8088` | Allowed browser origin for local API development |
| `DATADOCK_POOL_MAX_OPEN` | `10` | Maximum remote connections kept per database pool |

Treat the encryption key as a secret. Changing it prevents DataDock from decrypting previously saved connection passwords.

## Commands

```bash
make help
make build
make up
make logs
make down
make check
```

For local development, run the API and website in separate terminals:

```bash
make api
make web
```

## Persistence and backup

Docker stores DataDock metadata in the named `datadock-data` volume. Back it up before upgrading or moving hosts:

```bash
docker run --rm -v datadock_datadock-data:/data -v "$PWD":/backup alpine tar czf /backup/datadock-data.tar.gz -C /data .
```

Keep the backup and its encryption key together in secure storage. To intentionally remove all DataDock metadata, run `make clean`.

## Deployment safety

DataDock has no application login in this release. Publish it only on a trusted network, behind a VPN, or through an access-controlled reverse proxy. Do not expose the default HTTP port directly to the public internet. The app container only exposes the web gateway; the API remains on the internal Compose network.
