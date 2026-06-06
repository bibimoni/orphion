# Orphion Infrastructure Design

**Status:** Approved direction, pending written-spec review  
**Date:** 2026-06-07

## 1. Runtime Topology

```text
Google Chrome + Migaku
          |
          | http://127.0.0.1:<port>
          v
+-----------------------------------------------+
| Orphion application container                 |
|                                               |
| React static assets                           |
| Gin API                                       |
| Provider registry                             |
| Playback orchestration and restricted proxy   |
| SRT validation, parsing, and WebVTT generation|
| Progress and library services                 |
+-----------------------+-----------------------+
                        |
              +---------+----------+
              |                    |
              v                    v
     PostgreSQL container    Mounted app data
                             /var/lib/orphion
                                  |
                                  +-- subtitles/

Orphion provider adapter
          |
          v
Unofficial external anime source
```

The browser communicates only with Orphion. Provider details and upstream
media URLs remain behind the backend boundary.

## 2. Container Layout

### Development Compose

`app`

- Go toolchain.
- Node.js toolchain.
- Migration tooling.
- Mounted repository.
- Mounted application data directory.
- Runs backend and Vite development processes.

`db`

- PostgreSQL.
- Health check.
- Configurable host bind mount for development database data.
- Not published to the host unless explicitly enabled for debugging.

The app port is published as:

```text
127.0.0.1:<host-port>:<container-port>
```

### Production Compose

`app`

- Minimal runtime image.
- One compiled Go binary.
- Embedded or colocated React build.
- Read-only application/config files where practical.
- Writable `/var/lib/orphion` mount.

`db`

- PostgreSQL with a named volume.
- Internal Compose network access only.

## 3. Host Storage

Recommended application-data defaults:

| Platform | Host path |
|---|---|
| macOS | `~/Library/Application Support/Orphion` |
| Linux | `${XDG_DATA_HOME:-~/.local/share}/orphion` |
| Container | `/var/lib/orphion` |

Directory structure:

```text
Orphion/
├── subtitles/
├── logs/                 # optional file logging
└── runtime/              # optional non-durable runtime files
```

PostgreSQL data is not mixed with uploaded subtitle data. Development uses a
dedicated host directory resolved by `scripts/create_docker`, for example:

```text
~/Library/Application Support/Orphion/dev/postgres
```

Production uses a dedicated named Docker volume by default unless the operator
explicitly configures a bind mount.

The setup script resolves `~` and XDG paths before Compose starts because
Compose interpolation should not be relied on to expand shell home-directory
syntax in arbitrary values.

## 4. Configuration

Expected files:

```text
config/
├── config.example.yaml   # checked in
└── config.yaml           # ignored, mounted at runtime
```

Proposed shape:

```yaml
server:
  host: 127.0.0.1
  port: 8080

database:
  host: db
  port: 5432
  name: orphion
  user: orphion
  password_file: /run/secrets/postgres_password
  ssl_mode: disable

storage:
  root: /var/lib/orphion
  subtitle_directory: subtitles
  max_subtitle_bytes: 5242880

providers:
  default: allanime
  enabled:
    - allanime

playback:
  session_ttl: 30m
  connect_timeout: 5s
  response_header_timeout: 10s
  request_timeout: 30s
  max_redirects: 3
  max_manifest_bytes: 2097152

progress:
  save_interval: 10s
  completion_percent: 0.90
  completion_remaining: 2m

logging:
  level: info
  format: text
```

Exact keys may be refined during implementation planning, but configuration
must remain typed, validated on startup, and documented.

Secrets do not belong in committed YAML. Docker secrets or an ignored local
secret file supply the PostgreSQL password.

## 5. Network and Proxy Policy

The application listens on localhost by default. PostgreSQL remains on the
private Compose network.

The media proxy accepts only opaque session and resource IDs. It does not
offer a general-purpose URL proxy endpoint.

For every upstream request:

1. Retrieve the server-owned resource record.
2. Verify that the playback session is active.
3. Verify scheme and provider host policy.
4. Resolve DNS and reject loopback, private, link-local, multicast, and other
   prohibited ranges.
5. Apply provider-owned request headers.
6. Validate each redirect using the same policy.
7. Enforce time and size limits.
8. Stream the response without buffering full media segments.

Manifest responses are parsed and rewritten before returning to the browser.
Non-manifest media responses are streamed with controlled headers.

## 6. Database Operations

Migrations are versioned SQL files embedded in or shipped with the application
image.

Operational rules:

- `scripts/migrate up` applies migrations.
- `scripts/migrate status` reports schema state.
- App startup verifies that the expected schema is present.
- App startup does not automatically apply migrations in production.
- The default profile seed is idempotent.
- Development and integration tests use separate databases.

Backup guidance for Phase 1:

- Back up the PostgreSQL volume/database.
- Back up the Orphion subtitle directory.
- Restore both to preserve metadata-to-file consistency.

Automated scheduled backups are outside Phase 1, but the operations
documentation must include manual commands.

## 7. Development Scripts

### `scripts/create_docker`

- Detect macOS or Linux.
- Resolve the default host application-data path.
- Create required application-data and PostgreSQL-data directories.
- Create an ignored Docker environment file for resolved volume paths.
- Build development images.

### `scripts/run_docker`

- Start PostgreSQL and the app development container.
- Wait for database health.
- Enter or attach to the app development environment as appropriate.

### `scripts/install`

- Run inside the app container.
- Download Go modules.
- Install locked frontend dependencies.
- Install development tooling declared by the project.

### `scripts/run`

- Run inside the app container.
- Start Gin and Vite watchers.
- Forward termination signals and return non-zero when either process fails.

### `scripts/test`

- Run deterministic backend, frontend, integration, and browser-fixture tests.
- Keep live-provider smoke tests behind an explicit flag.

### `scripts/migrate`

- Run migration commands against the configured database.

Scripts must be POSIX-compatible where practical. Platform-specific path
handling belongs in small, explicit branches so Windows support can be added
later without rewriting application behavior.

## 8. Build Pipeline

The production Dockerfile uses stages:

1. `frontend-deps`: install locked dependencies.
2. `frontend-build`: build static React assets.
3. `go-build`: compile a statically linked or minimal-runtime-compatible Go
   binary with version metadata.
4. `runtime`: copy the binary, migrations, assets, and default configuration
   reference into a non-root runtime image.

The runtime image:

- Runs as a non-root user.
- Has no Go or Node toolchain.
- Exposes only the application port.
- Uses a health endpoint that checks process readiness and database
  connectivity without contacting the live provider.
- Writes only to `/var/lib/orphion` and explicitly writable temporary paths.

### Version policy

- Every Docker `FROM` instruction uses an explicit versioned tag.
- Every Compose `image` uses an explicit versioned tag.
- `:latest` and omitted image tags are prohibited.
- Go dependencies are pinned in `go.mod` and `go.sum`.
- Frontend dependencies are pinned by `package.json` and the committed
  package-manager lockfile.
- Node.js and Go toolchain versions are declared in repository files and
  matched by development and build images.
- Dependency upgrades are isolated, reviewed changes rather than incidental
  side effects of feature work.

## 9. Observability

Phase 1 uses structured application logs with:

- Request correlation ID.
- Stable error code.
- Provider key.
- Playback session ID where relevant.
- Upstream hostname, but not full sensitive URLs or query strings.
- Latency and response status.

Health endpoints:

- `/health/live`: process is running.
- `/health/ready`: configuration loaded, storage writable, and database
  reachable.

Provider health is not part of readiness because an unstable unofficial
provider must not cause the local application container to restart
continuously.

## 10. Phase Boundary

Phase 0 infrastructure is intentionally smaller:

```text
Static/disposable compatibility harness
        |
        +-- deterministic local video/HLS fixture
        +-- SRT upload
        +-- WebVTT track experiment
        +-- selectable DOM cue experiment
        +-- manual Migaku test report
```

It does not introduce PostgreSQL or the production Compose topology.

Only after the Phase 0 report is approved does Phase 1 create the application,
database, provider adapter, production proxy, and production images described
in this document.
