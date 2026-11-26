# Deployment Guide

## Prerequisites

- Go 1.26.5 or later (building from source)
- Node.js 18 or later **— build-time only** (Tailwind CSS generation; not required at runtime)
- Docker and Docker Compose (containerised deployment)
- olu v0.9.7 or later running and accessible

## Local development — make launch

The fastest way to get both services running locally:

```bash
make launch
```

See [README.md](README.md) for full launcher options.

## Docker Compose

```bash
cp config.example.json config.json
# Edit config.json — set jwt_secret and session_secret at minimum
docker-compose up -d
```

Starts Seam on port 8080 and xolu on port 9000. The compose file passes Seam the address of
xolu and the port in its environment, which wins over `config.json`, and starts xolu with the
settings `make launch` uses. This repository has no `Dockerfile` for the `build: .` of the
`seam` service, and the xolu image name in the file is not confirmed: the compose path has not
been run (T-124).

## Building from source

```bash
make build      # → bin/seam
./bin/seam -config config.json
```

## Self-contained binary and offline operation

The Seam AMS binary is fully self-contained. All frontend assets — Tailwind CSS,
HTMX, Material Icons font, Lit, and Leaflet — are embedded in the binary at
compile time via `//go:embed`. The binary makes no external network requests
for static assets after startup.

**Air-gapped deployments** work out of the box for all UI functionality except
the geographic map view, which fetches live tiles from OpenStreetMap at runtime.
To operate the map in an air-gapped environment, either:
- Deploy a self-hosted tile server and point Seam at it, or
- Disable the map: Settings → Features → Mapa de Activos (OFF).

See `web/static/vendor/VENDOR.md` for the complete asset inventory, exact
download URLs, checksums, and re-vendoring procedures.

**Node.js** is required at build time (to regenerate Tailwind CSS) but never at
runtime. The generated `web/static/css/tailwind.css` is committed to the
repository so the binary can be built from a clean checkout without Node:
```bash
go build ./cmd/seam   # works without Node; uses committed tailwind.css
```
Running `release.sh` (or `make css`) regenerates `tailwind.css` from the Go
source files using the Tailwind CLI.

## Configuration

### config.json

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "shutdown_timeout": "10s"
  },
  "olu": {
    "url": "http://localhost:9090",
    "timeout": "30s"
  },
  "auth": {
    "enabled": true,
    "jwt_secret": "change-this",
    "session_secret": "change-this-too",
    "token_expiry": "24h"
  },
  "features": {
    "enable_ui": true,
    "enable_api": true,
    "enable_fsm": true,
    "enable_qr": true,
    "dev_mode": false
  },
  "log": {
    "level": "info",
    "format": "json"
  }
}
```

### Environment variables

Environment variables take precedence over config file values.

| Variable | Description |
|---|---|
| `SEAM_PORT` | HTTP server port |
| `XOLU_URL` | xolu server URL |
| `SEAM_JWT_SECRET` | JWT signing secret |
| `SEAM_XOLU_API_KEY` | xolu API key (if xolu auth is enabled) |
| `SEAM_LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` |
| `SEAM_AUTH_ENABLED` | Enable/disable authentication (`true`/`false`) |

### xolu settings Seam needs

Photo galleries (the Photos field of the Assets form) are stored in xolu's blob store, which is
**off by default**. Start xolu with `XOLU_BLOB_ENABLED=true`; `scripts/launch.sh`, the start
scripts of the demo bundle and `docker-compose.yml` already do. Without it, every upload fails
with "upload failed" (xolu answers its blob requests with `XOLU-BL001`, 501). Its default limit per
blob (`XOLU_BLOB_MAX_SIZE`, 64 MB) is above the 20 MB the gallery accepts.

## Production

### Feature flags and the Developer module

Every on/off feature is a flag in one catalogue (`internal/features`). A flag's value comes, in
increasing priority, from its default, `SEAM_PROFILE`, the config file (the `features` keys, or
the generic `flags` map), its own environment variable (for example `SEAM_ENABLE_DEMO=1`) and
`SEAM_FEATURES` (`developer,demo,-work_orders`). A bad value stops the server at startup.
`features.select_search_min` and `features.row_effects` are plain settings, not flags.

The Developer module is off by default. To use it:

1. Turn authentication on (`SEAM_AUTH_ENABLED=1` and a `SEAM_JWT_SECRET`); the module refuses to
   start without it.
2. Set `SEAM_ENABLE_DEVELOPER=1` (or `SEAM_FEATURES=developer`). `SEAM_PROFILE=production`
   refuses it unless `SEAM_ALLOW_DEV_IN_PRODUCTION=1`.
3. Create the first developer with `seam-admin user create --email=... --name=... --role=developer`,
   or promote a user with `seam-admin user set-role --email=... --role=developer`. After that a
   developer can create developers, and change a user's role to or from developer, in the user
   forms. Nobody else is offered the role, and a developer cannot remove their own.
4. Log in as that user and open `/developer` (it is also in the module selector).

Two optional directories hold overrides. `SEAM_LOCALES_DIR` takes `<locale>.json` files that list
only the keys to override. `SEAM_SCHEMAS_DIR` takes `<module>/<name>.json` form definitions, where
the module is a directory under `internal/formengine/schemas` (`system`, `masters`, `issues`,
`sensors`, `workorders`, `asset`). Every file is checked before it takes effect, and a broken file
never replaces the last good one. A form-definition override replaces the whole form on the
production pages (not the Demo's examples). The per-tenant switches on the page (filesystem and
database overrides, show translation keys) apply to page bodies and to the navigation alike, and
while one that changes what a tenant's users see is on, every page of that tenant says so.

Overrides can also be stored per tenant, in the database. This is a decision that is off until it is chosen (`developer_tenant_overrides`, below). With it on, the editors offer two scopes: "This
tenant", kept in two xolu entities (`i18n_override`, and `schema_override`, which keeps every
version of a form definition with a note and can restore an old one), and "All tenants", the files
above. A file override outranks a tenant's, and the editors say when one hides the other. Whether
xolu needs those two entities declared before first use is not confirmed; if reading them fails at
startup the server logs it and starts without tenant overrides.

With several Seam instances, each loads the stored overrides and switches at startup and then
checks for another instance's changes every 15 seconds (a check reads the switch rows and the two
override entities), so a change made on one is live on the others within about that time. The
files in `SEAM_LOCALES_DIR` and `SEAM_SCHEMAS_DIR` are read from each instance's own disk, so they
must be the same on every instance.

A developer can also look at the application as a user with a lower role would: the Overview has a
"View as a role" section, every page then carries a notice naming the role with a link to leave,
and the Developer module stays reachable. Where Accounting is on, its three roles (admin staff,
accountant, account executive) are offered too. It only works for a developer, and only while the
Developer module is on.

For development, `make dev` starts everything with the Developer and Demo modules on, per-tenant
overrides (`developer_tenant_overrides`) and the form designer (`developer_form_designer`) on, and the
override directories under `data/dev/`. It also turns on Work Orders, Issues and Accounting, and on the first
run of a data directory loads a dataset (`SEED=`, by default `01-constructec`) with its work orders,
issues and accounting, so there is a developer login (`dev@ual.li`) and data in every module.
Its password is published in the seed scripts and the catalogue: change it or remove it on any
instance others can reach.

#### Decisions of the Developer module

Each choice below is a flag, so a deployment can choose differently. They all need the Developer
module on, they are all of the developer class (the `production` profile refuses them), and they are
set like any flag: its own environment variable, `SEAM_FEATURES` (`developer,developer_tenant_overrides`),
or the `flags` map of `config.json`. The `development` profile turns the Developer module on, with
tenant overrides and the form designer.

| Flag (environment variable) | Default | Decides | Why that default |
|---|---|---|---|
| `developer_view_as` (`SEAM_DEVELOPER_VIEW_AS`) | on | Whether a developer can view the application as a lower role. | It can only lower privilege, only for a developer, only while the module is on; it is how roles get tested; and the notice that it is active is always shown. |
| `developer_tenant_overrides` (`SEAM_DEVELOPER_TENANT_OVERRIDES`) | off | Whether translation and form-definition overrides are stored per tenant in xolu and applied. | It changes what a tenant's real users see, it needs two xolu entities that have not been checked against a live xolu, and every instance reads them. Opt in until that is verified. |
| `developer_instance_sync` (`SEAM_DEVELOPER_INSTANCE_SYNC`) | on | Whether instances check for each other's changes to the stored switches and overrides. | Instances that disagree about stored overrides are a correctness hazard, and a check every 15 seconds is cheap. |
| `developer_change_notice` (`SEAM_DEVELOPER_CHANGE_NOTICE`) | on | Whether every page of a tenant says so while a switch that changes what its users see is on. | It stops translation keys, or a layer switched off, being mistaken for a defect. Off is for demos and screenshots. |
| `developer_protect_own_role` (`SEAM_DEVELOPER_PROTECT_OWN_ROLE`) | on | Whether a developer is refused removing their own developer role. | It prevents locking the module out of its only developer. Off is for throwaway sandboxes. |
| `developer_form_designer` (`SEAM_DEVELOPER_FORM_DESIGNER`) | off | Whether the form designer is available. | New and incomplete. |
| `designer_live_preview` (`SEAM_DESIGNER_LIVE_PREVIEW`) | on | Whether the designer renders the form after every change. | Cheap, and it is what makes the designer usable; off is for forms so large that rendering each time is slow. |
| `developer_module_switches` (`SEAM_DEVELOPER_MODULE_SWITCHES`) | on | Whether the panel can switch the Demo, Work Orders and Issues modules on for one tenant at runtime. | Nothing changes until a developer uses a switch. Off leaves those modules' routes unmounted unless their own flag is on. |

Settings that are not on/off (0 or empty is the default; the Startup page of the module lists the values in force):

| Setting | Default | What it decides |
|---|---|---|
| `features.developer_sync_seconds` (`SEAM_DEVELOPER_SYNC_SECONDS`) | 15 | How often an instance checks what another changed; 2 to 3600. |
| `features.developer_poll_ms` (`SEAM_DEVELOPER_POLL_MS`) | 2000 | How often an override directory is loaded while its hot-load switch is on; 100 to 60000. |
| `features.developer_history_ms` (`SEAM_DEVELOPER_HISTORY_MS`) | 250 | How often an override directory is looked at for the history of changes made in the files; 20 to 60000, or -1 for only when it is loaded. |
| `features.developer_global_tenant` (`SEAM_DEVELOPER_GLOBAL_TENANT`) | 0 | The tenant whose developers may edit overrides that apply to every tenant; 0 is nobody. |
| `features.developer_source_dir` (`SEAM_DEVELOPER_SOURCE_DIR`) | empty | A source checkout scanned for texts no code refers to; empty hides that list. |
| `features.developer_watch` (`SEAM_DEVELOPER_WATCH`) | `auto` | How the override directories are followed: `auto` uses file-system events where the platform has them (Linux) and polling elsewhere (macOS); `events` or `poll` force one. Polling stays on beside events. |
| `features.developer_module_audience` (`SEAM_DEVELOPER_MODULE_AUDIENCE`) | `developers` | Who a module switched on from the panel is on for: the tenant's developers, or every user of the tenant (`tenant`). |
| `features.developer_preferences` (`SEAM_DEVELOPER_PREFERENCES`) | `account` | Where a developer's last scope and language are kept: with the account, or in the browser only (`browser`). |
| `features.developer_custom_pages` (`SEAM_DEVELOPER_CUSTOM_PAGES`) | off | Whether a form created with the module has a page (`/forms/{module}/{name}`) any user of the tenant can open and submit; submissions are stored as `form_submission`. |

Not choices, so not flags: the tenant of an override always comes from the session, a file override
outranks a tenant's, and file and database overrides share their validation.

`make launch-dev` is unrelated to all of this: it only sets `dev_mode`, which turns on two
development fallbacks and a startup warning. It takes effect on every launch, also on a data
directory whose `config.json` was written without it. Neither `make launch` nor `make launch-dev`
enables the Developer module; set its variables in the environment before running them.


### Static file compression

Seam gzips text-like files under `/static/` itself (JS, CSS, SVG, JSON) for clients that
accept it, so no proxy setting is needed for that. Set `features.compress_static` to
`false` (env `SEAM_COMPRESS_STATIC=0`) if your reverse proxy already compresses and you
want it to be the only layer. Range requests, HEAD requests and already-compressed files
(fonts, images) are always served as they are.

### Reverse proxy (nginx)

```nginx
server {
    listen 80;
    server_name seam.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name seam.example.com;

    ssl_certificate     /etc/ssl/certs/seam.crt;
    ssl_certificate_key /etc/ssl/private/seam.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Health check

```bash
curl http://localhost:8080/health
```

Returns `200 OK` when Seam is running. There is no separate readiness
endpoint — health is a combined liveness check.

### Logging

Logs are written to stdout in JSON format:

```json
{"time":"2026-03-14T12:00:00Z","level":"INFO","msg":"server started","port":8080}
```

### Metrics

Prometheus metrics integration is planned but not yet implemented.

### Scaling

Seam is stateless and can be horizontally scaled behind a load balancer.
All instances must point to the same olu server. With the Developer module on, instances stay in
step through the database within about 15 seconds (see above); the override directories are per
instance.

## Troubleshooting

**Connection refused to olu**
- Verify olu is running: `curl http://localhost:9090/health`
- Check `XOLU_URL` or `config.json → xolu.url`

**Photo upload fails with "upload failed"**
- xolu was started without its blob store: set `XOLU_BLOB_ENABLED=true` and restart xolu
  (see "xolu settings Seam needs" above)

**JWT authentication failures**
- Ensure `jwt_secret` is consistent across restarts
- Check token expiry settings

**Debug mode**
```bash
SEAM_LOG_LEVEL=debug ./bin/seam -config config.json
```
