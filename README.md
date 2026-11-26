# Seam AMS

**Current version:** 0.4.5 Management System built on [olu](https://github.com/ha1tch/xolu) —
a graph-enhanced document store with SQL-like query capabilities.

## Quick start

### Prerequisites

- Go 1.26.5 or later
- Git
- `curl` (for health checks in the launcher)

### Launch with one command

```bash
git clone https://github.com/ha1tch/seam.git
cd seam
make launch
```

This clones and builds the latest [xolu](https://github.com/ha1tch/xolu),
builds Seam AMS, generates secrets, writes config, and starts both services.
Seam is available at **http://localhost:8080**. Press Ctrl-C to stop.

### Options

```bash
make launch-fast       # skip rebuild (use existing binaries)
make launch-dev        # Seam's dev mode
make dev               # every module on (Developer, Demo, Work Orders, Issues, Accounting) over a dataset
make launch SEED=03-extinfire        # start with that dataset, its work orders, issues and accounting, those modules on
make launch MODULES=issues,work_orders   # switch on modules that are off by default: all, none, or a list
make launch-accounting # Accounting as the whole system, without the Assets module (data/accounting-only)
make dev-accounting    # the same with the Developer module on: view Accounting as each of its roles
make demo-accounting   # Accounting alone as a self-contained package
make launch SEED=none ARGS="--yes"   # start empty and ask nothing
make start-xolu        # xolu alone, as Seam needs it
make demo              # a self-contained package with data, for a machine with nothing installed
make help              # everything else

# or with custom ports:
./scripts/launch.sh --seam-port 8090 --xolu-port 9091
```

The first run of a data directory asks what the instance should start with: empty, with an
administrator you create, or one of seven demonstration datasets with accounts to sign in with.
`make launch`, `make dev`, `make start-xolu` and `make demo` start xolu with the same settings
(`scripts/lib/xolu-env.sh`) and load data the same way (`scripts/lib/seeds.sh`).

### Build only

```bash
make build             # → bin/seam
make test
make vet
```

### Distributable bundle

```bash
make bundle            # → seam-ams-VERSION-bundle.zip
```

Produces a zip that a recipient can unzip and run `./launch.sh` on without
any prior setup beyond Go and Git.

## Configuration

The launcher writes `data/seam/config.json` on first run. Edit it and
restart to apply changes.

Key settings:

| Setting | Default | Description |
|---|---|---|
| `server.port` | 8080 | HTTP port |
| `xolu.url` | http://localhost:9090 | xolu server URL |
| `auth.jwt_secret` | (generated) | JWT signing key |
| `features.enable_qr` | true | QR code generation |
| `features.dev_mode` | false | Enable dev mode |

Environment variables (take precedence over config file):

| Variable | Description |
|---|---|
| `SEAM_PORT` | HTTP server port |
| `XOLU_URL` | xolu server URL |
| `SEAM_JWT_SECRET` | JWT signing secret |
| `SEAM_LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` |
| `SEAM_XOLU_API_KEY` | xolu API key (if xolu auth is enabled) |

### Feature flags and the Developer module

Optional features are switched with the `features` keys in `config.json`, or with these environment
variables (which win):

| Variable | Description |
|---|---|
| `SEAM_FEATURES` | Flags to turn on or off, for example `developer,demo,-work_orders` |
| `SEAM_PROFILE` | A named set of defaults (`production` refuses the Developer module) |
| `SEAM_ENABLE_DEVELOPER` | Turn on the Developer module (needs authentication) |
| `SEAM_ALLOW_DEV_IN_PRODUCTION` | Allow the Developer module under the production profile |
| `SEAM_LOCALES_DIR` | Directory of translation override files |
| `SEAM_SCHEMAS_DIR` | Directory of form-definition override files |

See [DEPLOY.md](DEPLOY.md#feature-flags-and-the-developer-module) for what each does.

## Docker

```bash
docker-compose up -d
```

Starts both Seam and xolu. See [DEPLOY.md](DEPLOY.md) for production guidance.

## API

Base URL: `/api/v1`

| Endpoint | Description |
|---|---|
| `/assets` | Asset CRUD |
| `/asset-types` | Asset type management |
| `/events` | Event ingestion |
| `/fsm` | FSM machine management |
| `/qr` | QR code management |
| `/rules` | Rules engine |
| `/alerts` | Alert management |
| `/reports` | Analytics and export |

See `api/openapi.yaml` for the full specification.

## Health

```bash
curl http://localhost:8080/health
```

## License

MIT — Copyright (c) 2026 haitch
