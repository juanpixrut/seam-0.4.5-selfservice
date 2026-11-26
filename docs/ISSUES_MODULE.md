# Issues module

Reporting and following up what goes wrong with an asset: an issue is reported from the
application or from a QR code on the asset, assigned to a person, worked through a lifecycle
with response and resolution deadlines, and closed. The screens are `internal/ui/issue*.go`;
`internal/modules/issues` registers them.

Issues began as a small tracker and is now an application module. This document describes
it as it is and lists, at the end, what it lacks to meet what the other application modules
of Seam meet. Each gap is an item in `docs/TRACKING.md`.

## Switching it on

Issues is off unless it is switched on, the way Work Orders is:

| How | What it does |
|---|---|
| `SEAM_ENABLE_ISSUES=1`, or `"enable_issues": true` under `features` in `config.json` | On for every tenant, from startup. The flag is `issues` in `internal/features/catalog.go`, class preview. |
| The switch **Issues module** in the Developer panel (`module_issues`) | On for one tenant, without a restart, while the flag is off: for that tenant's developers, or for all its users with `developer_module_audience` set to `tenant`. Needs the Developer module and `developer_module_switches`. |

Where it is off its routes answer 404 and the menus have no entry. The page of a scanned QR
code (`/s/{code}`) follows the flag alone, not the switch, since whoever scans has no tenant:
with the flag off it shows the asset and offers no form. The QR card on an asset's page belongs
to Assets and works either way.

`make launch SEED=...`, `make demo` and `make dev` switch it on together with the data they
load (`scripts/lib/modules.sh`); `MODULES=issues` does it for an instance without that data.

It needs two things of xolu, both set by `scripts/lib/xolu-env.sh` for every way this
repository starts xolu:

| Setting | Why |
|---|---|
| `XOLU_API_V2_ENABLED=true` | The lifecycle of an issue is a xolu state machine (`/api/v2/fsm`). Without it the list opens and reporting an issue answers 500. |
| `XOLU_BLOB_ENABLED=true` | The images of the QR codes are stored as blobs. |

## What it holds

| Entity | What it is | Found by |
|---|---|---|
| `issues` | One issue: title, description, priority, severity, state, the asset, who reported it, who is responsible, deadlines, the template's values | id |
| `issue_templates` | Extra fields asked when reporting, with versions; one version of a key is current | `key`, `version` |
| `issue_template_assignments` | Which template an asset type uses | `asset_type_id` |
| `issue_qr_codes` | The code printed on an asset; a revoked code is kept | `code` |

Each issue has a state machine of its own in xolu (`machine_id`), made from the definition
`seam_issue_lifecycle`, which the module creates on first use and finds by name afterwards.
The history shown on an issue is the history of its machine.

## Lifecycle

| State | Reached by | From |
|---|---|---|
| `reported` | reporting | — |
| `triaged` | `triage`: sent when a reported issue is assigned | `reported` |
| `in_progress` | `start` | `reported`, `triaged` |
| `resolved` | `resolve` | `in_progress` |
| `closed` | `close` | `resolved` |
| `rejected` | `reject`, with a reason | any state that is not final |
| `on_hold` | `hold` | `triaged`, `in_progress` |
| `duplicate` | `merge` | `reported`, `triaged`, `in_progress` |

`closed`, `rejected` and `duplicate` are final. The definition has `hold`, `resume` and
`merge`; the screens send none of them, so `on_hold` and `duplicate` cannot be reached from
the application (T-126).

## Priority, severity and deadlines

Priority is `critical`, `high`, `medium` or `low`; severity is `critical`, `major` or
`minor`. An issue on an asset takes a category from the asset's criticality: `critico` when it is
high, `normal` otherwise. Two
deadlines are stored when the issue is reported, in minutes from that moment:

| | critical | high | medium | low |
|---|---|---|---|---|
| First response, critical asset | 60 | 120 | 240 | 240 |
| First response, other | 120 | 240 | 480 | 480 |
| Resolution, critical asset | 240 | 1440 | 1440 | 2880 |
| Resolution, other | 1440 | 2880 | 4320 | 10080 |

A template may carry times of its own, which replace these for the issues reported with it.
The first response is the moment the issue is started for the first time; that time and
who started it are stored.

## Templates

A template is a set of fields (text, long text, number, yes or no, date, date and time)
asked in addition to the standard ones. It is assigned to asset types: reporting an issue on an asset of that type
shows its fields, and the answers are stored with the issue together with the version they
were asked with. Editing a template makes a new version; issues keep the one they have.

## QR codes

An asset can have a code. Its page (`/s/{code}`) is public: anyone who scans it can report
an issue on that asset without an account. Such an issue has the channel `qr`, no reporter
and no responsible. A code can be revoked and replaced; the revoked one stops working and
stays in the list.

## Cases

Issues about one cause can be grouped under one of them, the case. The grouped issues then
follow the case: its responsible is theirs, a step taken on the case is taken on each, and
they cannot be acted on one by one. Ungrouping gives an issue back its own course.

## Who can do what

Access is decided in the handlers, by role rank and by who the issue belongs to. The module
has no permissions of its own (T-125).

| Action | Who |
|---|---|
| See the list and any issue, report an issue | Any signed-in user, of any role |
| Start, resolve, close, reject | The responsible of the issue, and administrators. If nobody is responsible: any signed-in user |
| Assign a responsible | Administrators, until the issue is started |
| Group and ungroup | Administrators |
| Templates, the list of QR codes | Roles with `settings.edit` |
| Make, replace or revoke the code of an asset | Roles with `assets.asset.edit` |
| Report from a QR code | Anyone, without an account |

"Administrators" is the role `admin` and those above it (`developer`).

## Demonstration data

`seeds/issues/seed-issues-NN-name.seed`, one for each dataset: 12 issues over the whole
lifecycle, one case, 2 templates, 4 QR codes. `make launch SEED=...`, `make dev` and the demo
packages load it with the dataset; by hand:

```bash
python3 seeds/seed.py load issues-01-constructec --url http://localhost:9090
```

The history of a seeded issue carries the time of the load, since its steps are made then.

## What is missing for a proper application module

| Gap | Today | Item |
|---|---|---|
| Module lifecycle | Done in 0.4.5: Issues has the flag, the switch for one tenant, the gated routes and the menu entry that Work Orders has (see "Switching it on"). | T-131 |
| Permissions | No `issues.*` permissions. A `viewer`, whose role is read-only, can report, and can start, resolve or reject an issue nobody is responsible for; so can the roles of the Accounting module (`account_executive`, `accountant`, `admin_staff`), which also see the module in the navigation. Checked on a running instance, 2026-11-18. | T-125 |
| Lifecycle | `hold`, `resume` and `merge` are defined and cannot be sent. | T-126 |
| Audit log | Nothing in Issues writes to the audit log; assets, settings and the Developer module do. What happened to an issue is in its machine's history only, and a change of responsible, a grouping or a template change is recorded nowhere. | T-127 |
| What it needs of xolu | Seam does not check at start that xolu serves `/api/v2`; the failure appears as a 500 on the first report. | T-128 |
| Stored values | The asset category is stored as `critico`, a Spanish word used as a constant. Internal values are English. | T-129 |
| Dashboard and API | Assets and Documents have a dashboard, and assets a REST API; Issues has neither. | T-130 |
| Template keys | A template whose name has no Latin letters or digits is refused. | T-122 |
