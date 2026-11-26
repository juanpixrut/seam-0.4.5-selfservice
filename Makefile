# Makefile — Seam AMS
#
#   make help        every target, with what it does
#
# The help is written from this file: a line "##@ Title" starts a section,
# "target: ... ## text" describes a target, and "##> text" is a note under the
# section it is in. A target without "##" is internal and is not listed.
# scripts/tests/test_makefile.py holds the file to that: every target in
# .PHONY is described, and what the launch targets start is the same system.
#
# The three ways to a running system share their parts, so they cannot drift:
#   scripts/lib/xolu-env.sh   how xolu is started (launch, demo, demo packages)
#   scripts/lib/seeds.sh      which data is loaded, and how
#   scripts/lib/ui.sh         what the scripts print

.PHONY: help build test test-workflow test-seeds test-seeds-live test-scripts seed-accounting vet check check-icons \
        i18n-check i18n-todo css vendor-js \
        launch launch-dev launch-fast launch-accounting demo-accounting dev dev-accounting xolu start-xolu olu start-olu \
        admin-create user-create user-list \
        user-disable user-enable user-set-role user-set-password user-delete \
        bundle demo release clean

.DEFAULT_GOAL := help

# The order of the sections in `make help`:
##@ Launch
##@ Development
##@ User management
##@ Distribution
##@ Housekeeping

VERSION := $(shell cat VERSION 2>/dev/null || echo "unknown")

# Where xolu answers, for the user-management targets:
#   make user-list XOLU_URL=http://host:port     (OLU_URL, the old name, still works)
XOLU_URL ?= $(or $(OLU_URL),http://localhost:9090)

# Passed to scripts/launch.sh by the launch targets:
#   SEED=03-extinfire   load that dataset and its additions (first run of a data directory)
#   SEED=none           start empty without being asked
#   MODULES=issues,work_orders   the modules that are off by default to switch
#                       on: all, none, or a list (scripts/lib/modules.sh). Not
#                       given, a dataset switches on the modules its data is for
#   ARGS="--yes"        any other option of scripts/launch.sh (see its --help)
# make demo takes SEED and MODULES the same way.
SEED ?=
MODULES ?=
ARGS ?=
LAUNCH_ARGS = $(if $(SEED),--seed $(SEED)) $(if $(MODULES),--modules $(MODULES)) $(ARGS)

# ── Build ─────────────────────────────────────────────────────────────────────
##@ Development

build: ## Build bin/seam and bin/seam-admin
	@echo "Building Seam AMS $(VERSION) ..."
	@mkdir -p bin
	go build -o bin/seam       ./cmd/seam
	go build -o bin/seam-admin ./cmd/seam-admin
	@echo "  → bin/seam"
	@echo "  → bin/seam-admin"

# ── Test ──────────────────────────────────────────────────────────────────────

test: ## Run the tests: Go, the seed loader, the scripts
	go test ./...
	python3 -m unittest discover -s seeds/tests
	python3 -m unittest discover -s scripts/tests

# test-workflow runs end-to-end workflow tests against an embedded xolu server.
# These tests live in internal/ui/workflow/ and use a separate go.mod that
# imports xolu as a test-only dependency. xolu must be present at /tmp/olu
# (the replace directive points there) or XOLU_PATH must be overridden:
#   make test-workflow XOLU_PATH=/path/to/xolu
#
# The main seam-ui module (go.mod) is NOT affected by this target.
XOLU_PATH ?= /tmp/olu
test-workflow: ## End-to-end workflow tests against an embedded xolu (XOLU_PATH=)
	@echo "Running workflow tests (embedded xolu @ $(XOLU_PATH)) ..."
	@if [ ! -d "$(XOLU_PATH)" ]; then \
		echo "  ERROR: xolu not found at $(XOLU_PATH)"; \
		echo "  Set XOLU_PATH=/path/to/xolu or ensure /tmp/olu is present."; \
		exit 1; \
	fi
	@# Patch the replace directive if XOLU_PATH differs from /tmp/olu.
	@# Must resolve to absolute path because go test runs from internal/ui/workflow/.
	@if [ "$(XOLU_PATH)" != "/tmp/olu" ]; then \
		abs=$$(cd "$(XOLU_PATH)" && pwd) && \
		python3 -c "import re,sys; p=sys.argv[1]; f='internal/ui/workflow/go.mod'; \
		t=open(f).read(); t=re.sub(r'github\.com/ha1tch/xolu => .*',\
		'github.com/ha1tch/xolu => '+p,t); open(f,'w').write(t)" "$$abs"; \
	fi
	cd internal/ui/workflow && \
	    GONOSUMDB='*' GONOSUMCHECK='*' GOTOOLCHAIN=local \
	    go test ./... -v -timeout 120s

# test-seeds checks the seed loader (seeds/seed_system) and the .seed files
# without a server. test-seeds-live runs the conformance suite, which starts a
# real xolu for each case and so needs its binary:
#   make test-seeds-live XOLU_BIN=/path/to/xolu
test-seeds: ## Test the seed loader and the .seed files, without a server
	python3 -m unittest discover -s seeds/tests

XOLU_BIN ?=
test-seeds-live: ## Run the seed conformance suite on a real xolu (XOLU_BIN=)
	@if [ -z "$(XOLU_BIN)" ]; then \
		echo "  ERROR: set XOLU_BIN=/path/to/xolu (a xolu 0.30.44 or later)"; \
		exit 1; \
	fi
	XOLU_BIN="$(XOLU_BIN)" python3 seeds/conformance/run.py

# seed-accounting writes seeds/accounting/seed-accounting.seed again, from the
# accounting module's own rules (cmd/accounting-seedgen). Run it after a change
# to internal/accounting that changes what the module stores.
seed-accounting: ## Write the accounting seed again from the module's rules
	go run ./cmd/accounting-seedgen > seeds/accounting/seed-accounting.seed.new
	mv seeds/accounting/seed-accounting.seed.new seeds/accounting/seed-accounting.seed

# test-scripts checks what the scripts print (scripts/lib/ui.py: every box and
# table stays aligned, whatever the text) and that this Makefile, the launcher
# and the demo builder agree with each other. No server, no network.
test-scripts: ## Test the scripts' output library and the launch targets' consistency
	python3 -m unittest discover -s scripts/tests

vet: ## Run go vet
	go vet ./...

# ── Preflight ────────────────────────────────────────────────────────────────

# check runs all verifications without cutting any artefacts.
# Use this before release or after pulling changes. Safe to run any time.
check: check-icons i18n-check ## Full preflight before a release: versions, scripts, tests, vet, JS
	@echo ""
	@echo "=== Version consistency ==="
	./syncver.sh check
	@echo ""
	@echo "=== Shell scripts ==="
	shellcheck --format=gcc -x release.sh syncver.sh scripts/launch.sh scripts/build-demo.sh scripts/bundle.sh \
	    scripts/lib/ui.sh scripts/lib/xolu-env.sh scripts/lib/seeds.sh
	python3 -m unittest discover -s scripts/tests
	python3 -m unittest discover -s seeds/tests
	@echo ""
	@echo "=== Go tests ==="
	go test ./...
	@echo ""
	@echo "=== Go vet ==="
	go vet ./...
	@echo ""
	@echo "=== JS syntax ==="
	node --check web/static/js/app.js
	node --check web/static/js/fsm-editor.js
	node --check web/static/js/seam-fsm-editor.js
	@echo ""
	@echo "All checks passed."

# ── CSS / Icons ─────────────────────────────────────────────────────────────

# css regenerates web/static/css/tailwind.css from source.
# Required after adding or removing Tailwind classes in Go templates.
css: ## Regenerate web/static/css/tailwind.css
	@command -v node >/dev/null 2>&1 || (echo "ERROR: Node.js not found" && exit 1)
	@[ -f tailwind.config.js ] || python3 scripts/check-icons.py --write-config
	npx tailwindcss -c tailwind.config.js -i scripts/tailwind.input.css \
	    -o web/static/css/tailwind.css --minify
	@echo "  → web/static/css/tailwind.css"

# check-icons audits Material Icons usage against the vendored WOFF2 font.
# Regenerates web/static/vendor/material-icons-ligatures.txt.
# Run after adding new icons or updating the vendored font.
# Requires: pip install fonttools brotli
check-icons: ## Audit icon names against the vendored font
	@command -v python3 >/dev/null 2>&1 || (echo "ERROR: python3 not found" && exit 1)
	python3 scripts/check-icons.py

# i18n-check fails if the locale files do not all have the same keys.
# Adding keys to all four files at once: scripts/i18n-sync.py (see its --help).
i18n-check: ## Every locale file has the same keys
	python3 scripts/i18n-sync.py check

# i18n-todo lists values still carrying the [TODO] marker.
i18n-todo: ## Translations still marked [TODO]
	python3 scripts/i18n-sync.py todo

# ── Launch ────────────────────────────────────────────────────────────────────
##@ Launch
##> All of these run scripts/launch.sh. The first run of a data directory asks
##> what to start with: empty, or a dataset with work orders, issues and accounting.
##>
##> SEED=03-extinfire answers that; SEED=none starts empty. ARGS="--yes" asks nothing.
##>
##> Work Orders, Issues and Accounting are off unless switched on. A dataset switches
##> on the ones it has data for; MODULES=all, none or issues,work_orders says otherwise.
##>
##> Ports: SEAM_PORT=8090 XOLU_PORT=9091. Data directory: SEAM_DATA_DIR=...

launch: scripts/launch.sh ## Obtain xolu, build everything, start both services
	@chmod +x scripts/launch.sh
	./scripts/launch.sh $(LAUNCH_ARGS)

launch-fast: scripts/launch.sh ## The same without rebuilding what exists
	@chmod +x scripts/launch.sh
	./scripts/launch.sh --skip-build $(LAUNCH_ARGS)

launch-dev: scripts/launch.sh ## The same with Seam's dev mode on
	@chmod +x scripts/launch.sh
	./scripts/launch.sh --dev $(LAUNCH_ARGS)

# dev: everything a developer of Seam wants on, over a dataset.
#   SEAM_ENABLE_DEVELOPER  Developer module (/developer); needs auth, which
#                          launch.sh always enables
#   SEAM_ENABLE_DEMO       Demo module
#   SEAM_ENABLE_WORK_ORDERS, SEAM_ENABLE_ISSUES, SEAM_ENABLE_ACCOUNTING
#                          the three application modules that are off by
#                          default (scripts/lib/modules.sh), so that all the
#                          seeded data shows
#   SEAM_DEVELOPER_TENANT_OVERRIDES  overrides per tenant, kept in xolu ("This tenant")
#   SEAM_DEVELOPER_FORM_DESIGNER     the form designer (/developer/designer)
#   SEAM_LOCALES_DIR       translation overrides, <locale>.json files
#   SEAM_SCHEMAS_DIR       form-definition overrides, <module>/<name>.json
# On the first run of a data directory it loads DEV_SEED and its additions, so
# there is a developer account to sign in with (dev@ual.li / holaquetal) and
# data in every module; a data directory already in use is left as it is.
# Hot loading is switched on from the /developer panel. Overrides:
#   make dev SEED=06-asahikenki DEV_LOCALES_DIR=... DEV_SCHEMAS_DIR=...
DEV_SEED        ?= 01-constructec
DEV_LOCALES_DIR ?= $(CURDIR)/data/dev/locales
DEV_SCHEMAS_DIR ?= $(CURDIR)/data/dev/schemas

dev: scripts/launch.sh ## Every module on (Developer, Demo, Work Orders, Issues, Accounting) over a dataset
	@chmod +x scripts/launch.sh
	@mkdir -p "$(DEV_LOCALES_DIR)" "$(DEV_SCHEMAS_DIR)"
	SEAM_ENABLE_DEVELOPER=1 \
	SEAM_ENABLE_DEMO=1 \
	SEAM_ENABLE_WORK_ORDERS=1 \
	SEAM_ENABLE_ISSUES=1 \
	SEAM_ENABLE_ACCOUNTING=1 \
	SEAM_DEVELOPER_TENANT_OVERRIDES=1 \
	SEAM_DEVELOPER_FORM_DESIGNER=1 \
	SEAM_LOCALES_DIR="$(DEV_LOCALES_DIR)" \
	SEAM_SCHEMAS_DIR="$(DEV_SCHEMAS_DIR)" \
	SEAM_DEVELOPER_SOURCE_DIR="$(CURDIR)" \
	    ./scripts/launch.sh --seed $(or $(SEED),$(DEV_SEED)) $(ARGS)

# Accounting as the whole system: Seam without the Assets module
# (enable_operational off), with Accounting on and its own data, in a data
# directory of its own, which a later launch finds the same way. "/" goes to
# Accounting. Its people sign in with the accounts of the accounting seed,
# whose developer account (dev@ual.li) manages the users.
# The modules that can be run this way: modules_standalone, scripts/lib/modules.sh.
ACCOUNTING_DATA_DIR ?= $(CURDIR)/data/accounting-only

launch-accounting: scripts/launch.sh ## Accounting alone, without the Assets module, in data/accounting-only
	@chmod +x scripts/launch.sh
	./scripts/launch.sh --only accounting --data-dir "$(ACCOUNTING_DATA_DIR)" $(ARGS)

# dev-accounting: launch-accounting with what make dev adds for a developer.
# The accounting seed has the developer account (dev@ual.li / holaquetal),
# which can view the application as each role of Accounting from /developer;
# viewed as an account executive it has a portal of its own (Sandbox Demo Co.).
dev-accounting: scripts/launch.sh ## Accounting alone with the Developer module on, to view it as each role
	@chmod +x scripts/launch.sh
	@mkdir -p "$(DEV_LOCALES_DIR)" "$(DEV_SCHEMAS_DIR)"
	SEAM_ENABLE_DEVELOPER=1 \
	SEAM_DEVELOPER_TENANT_OVERRIDES=1 \
	SEAM_DEVELOPER_FORM_DESIGNER=1 \
	SEAM_LOCALES_DIR="$(DEV_LOCALES_DIR)" \
	SEAM_SCHEMAS_DIR="$(DEV_SCHEMAS_DIR)" \
	SEAM_DEVELOPER_SOURCE_DIR="$(CURDIR)" \
	    ./scripts/launch.sh --only accounting --data-dir "$(ACCOUNTING_DATA_DIR)" $(ARGS)

# xolu alone, with the settings every other target uses (scripts/lib/xolu-env.sh).
xolu: scripts/launch.sh ## Obtain xolu (release binary or source build); start nothing
	@chmod +x scripts/launch.sh
	./scripts/launch.sh --xolu-only --no-start $(ARGS)

start-xolu: scripts/launch.sh ## Start xolu alone, as Seam needs it (API v2, blobs, full text)
	@chmod +x scripts/launch.sh
	./scripts/launch.sh --xolu-only --skip-build $(ARGS)

# The names these two had when xolu was called olu.
olu: xolu
start-olu: start-xolu

# ── User management ───────────────────────────────────────────────────────────
##@ User management
##> xolu must be running. Elsewhere than localhost:9090: XOLU_URL=http://host:port
##>
##> Roles: developer admin manager operator viewer admin_staff accountant account_executive

bin/seam-admin:
	@$(MAKE) build

admin-create: bin/seam-admin ## Create an administrator (asks for everything)
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user create --role=admin

user-create: bin/seam-admin ## Create a user (asks for the role)
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user create

user-list: bin/seam-admin ## List the users
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user list

user-disable: bin/seam-admin ## Disable a user: EMAIL=user@example.com
	@[ -n "$(EMAIL)" ] || { echo "Usage: make user-disable EMAIL=user@example.com"; exit 1; }
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user disable --email=$(EMAIL)

user-enable: bin/seam-admin ## Enable a user: EMAIL=user@example.com
	@[ -n "$(EMAIL)" ] || { echo "Usage: make user-enable EMAIL=user@example.com"; exit 1; }
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user enable --email=$(EMAIL)

user-set-role: bin/seam-admin ## Change a role: EMAIL=user@example.com ROLE=manager
	@[ -n "$(EMAIL)" ] || { echo "Usage: make user-set-role EMAIL=user@example.com ROLE=manager"; exit 1; }
	@[ -n "$(ROLE)" ]  || { echo "Usage: make user-set-role EMAIL=user@example.com ROLE=manager"; exit 1; }
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user set-role --email=$(EMAIL) --role=$(ROLE)

user-set-password: bin/seam-admin ## Set a password: EMAIL=user@example.com
	@[ -n "$(EMAIL)" ] || { echo "Usage: make user-set-password EMAIL=user@example.com"; exit 1; }
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user set-password --email=$(EMAIL)

user-delete: bin/seam-admin ## Delete a user: EMAIL=user@example.com
	@[ -n "$(EMAIL)" ] || { echo "Usage: make user-delete EMAIL=user@example.com"; exit 1; }
	SEAM_XOLU_URL=$(XOLU_URL) ./bin/seam-admin user delete --email=$(EMAIL)

# ── Bundle ────────────────────────────────────────────────────────────────────
##@ Distribution

bundle: scripts/bundle.sh ## Zip of the source with a launcher: the recipient runs ./launch.sh
	@chmod +x scripts/bundle.sh
	./scripts/bundle.sh

# ── Demo ──────────────────────────────────────────────────────────────────────
# Build a self-contained demo package for external reviewers.
# Cross-compiles for all platforms, asks for the dataset and the target OS
# (or takes them: make demo SEED=03-extinfire PLATFORM=linux), loads the
# dataset and its additions into a database, and packages everything into
# demo/<name>/. The target machine needs zero dependencies.
PLATFORM ?=

demo: scripts/build-demo.sh ## Self-contained demo package with data: SEED= PLATFORM=windows|linux|mac MODULES=
	@chmod +x scripts/build-demo.sh
	./scripts/build-demo.sh $(if $(SEED),--seed $(SEED)) $(if $(PLATFORM),--platform $(PLATFORM)) $(if $(MODULES),--modules $(MODULES)) $(ARGS)

demo-accounting: scripts/build-demo.sh ## The same package with Accounting alone, without the Assets module: PLATFORM=
	@chmod +x scripts/build-demo.sh
	./scripts/build-demo.sh --only accounting $(if $(PLATFORM),--platform $(PLATFORM)) $(ARGS)

# ── Release ───────────────────────────────────────────────────────────────────

release: ## Cut a release checkpoint: VER=x.y.z
	@[ -n "$(VER)" ] || { echo "Usage: make release VER=0.1.0-patchedN"; exit 1; }
	./release.sh $(VER)

# ── Clean ─────────────────────────────────────────────────────────────────────
##@ Housekeeping
##> data/xolu/ the database · data/seam/config.json Seam's configuration · data/.secrets (never commit)

clean: ## Remove bin/, demo/build/, coverage.out and zips (data/ and demo packages stay)
	@rm -rf bin/
	@rm -rf demo/build/
	@rm -f coverage.out
	@rm -f *-bundle.zip *-checkpoint.zip *-checkpoint.sha256
	@echo "Cleaned. (Demo packages in demo/ preserved — rm -rf demo/ to remove.)"

# ── Help ──────────────────────────────────────────────────────────────────────

help: ## This list
	@python3 scripts/make-help.py Makefile "Seam AMS $(VERSION)"

# ── vendor-js ─────────────────────────────────────────────────────────────────
# Download pinned ESM builds of Rete.js + Lit to web/static/vendor/.
# Run this once (or when pinned versions change) to enable vendor_js mode.
# After downloading, set "vendor_js": true in config.json or run with
# SEAM_VENDOR_JS=true to serve these files instead of fetching from CDN.

VENDOR_DIR := web/static/vendor

vendor-js: ## Download the pinned JS dependencies to web/static/vendor/
	@mkdir -p "$(VENDOR_DIR)"
	@echo "Downloading vendored JS dependencies to $(VENDOR_DIR) ..."
	curl -fsSL "https://esm.sh/rete@2.0.4"                 -o "$(VENDOR_DIR)/rete@2.0.4.js"
	curl -fsSL "https://esm.sh/rete-area-plugin@2.0.3"     -o "$(VENDOR_DIR)/rete-area-plugin@2.0.3.js"
	curl -fsSL "https://esm.sh/rete-connection-plugin@2.0.3" -o "$(VENDOR_DIR)/rete-connection-plugin@2.0.3.js"
	curl -fsSL "https://esm.sh/rete-render-utils@2.0.2"    -o "$(VENDOR_DIR)/rete-render-utils@2.0.2.js"
	curl -fsSL "https://esm.sh/@retejs/lit-plugin@2.0.7"   -o "$(VENDOR_DIR)/retejs-lit-plugin@2.0.7.js"
	curl -fsSL "https://esm.sh/lit@3"                      -o "$(VENDOR_DIR)/lit@3.js"
	@echo ""
	@echo "  Done. To enable vendored mode:"
	@echo '    Set "vendor_js": true in data/seam/config.json, or'
	@echo "    start Seam with SEAM_VENDOR_JS=true"
	@echo ""
	@ls -lh "$(VENDOR_DIR)"
