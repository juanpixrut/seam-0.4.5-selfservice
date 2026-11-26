# Accounting & Admin module

Implements the Seam Admin design (Account Executives, collections, commissions,
settlements, simplified double-entry books). Package `internal/accounting` holds the
rules; `internal/ui/accounting_*.go` the screens; `internal/modules/accounting` the
registration.

## Switching it on and settings

Everything is off or conservative by default. `features.enable_accounting` (env
`SEAM_ENABLE_ACCOUNTING=1`) turns the module on; the rest lives under `accounting` in the
config file (env `SEAM_ACCOUNTING_*`).

The module reads nothing of Assets, so it can be the whole system: `features.enable_operational`
off (env `SEAM_ENABLE_OPERATIONAL=0`) together with `enable_accounting` on. Then the routes of
Assets are not mounted, the menus do not offer it, and `/` goes to `/accounting` (an account
executive lands on the portal). `make launch-accounting` starts exactly that over the accounting
seed, in `data/accounting-only`, and `make demo-accounting` packages it. The people of the seed
(back office, accountant, account executives) sign in as they are; the seed's developer account
(`dev@ual.li`) manages the users. With the Developer module on (`make dev-accounting`) that
account views the application as each role of Accounting from `/developer`; viewed as an
account executive it has a portal of its own, because the seed makes it an authorized one with
a customer (Sandbox Demo Co.). Documents, Settings and Administration have
no switch of their own and are still there for an administrator; the accounting roles are not
offered them.

| Setting (env suffix) | Default | Why this default |
|---|---|---|
| `docs_dir` (`DOCS_DIR`) | `data/accounting-docs` | invoices and payment proofs on disk |
| `tenant_id` (`TENANT_ID`) | `0` (no restriction) | right for a single-tenant install. The module's records are not tenant-scoped, so on a multi-tenant install set it to the tenant that is Seam itself; users of other tenants then get 403 |
| `ae_self_register` (`AE_SELF_REGISTER`) | `true` | the design is "register, then Seam authorizes"; the authorization is the gate. `false` limits the portal to AEs Seam already knows |
| `accountant_operate` (`ACCOUNTANT_OPERATE`) | `false` | separation of duties: whoever keeps the books does not also move the money. `true` lets the accountant sync, collect and pay settlements |
| `staff_post_entries` (`STAFF_POST_ENTRIES`) | `false` | free ledger entries are the accountant's. `true` lets admin_staff post and reverse them. Closing a month stays with the accountant and the admin |
| `staff_edit_ae_profiles` (`STAFF_EDIT_AE`) | `false` | where the money goes is the AE's own statement; a back-office user who could rewrite payout details could redirect a payment. `true` lets admin_staff edit AE profiles (the admin always can). Every profile change is logged, and a change to payout method or details is marked `payout_changed` |
| `notify_platform` (`NOTIFY_PLATFORM`) | `true` | tells the platform when an item is collected; only has an effect with `instance_url`. Without a platform no warning or retry button is shown. `false` for platforms that learn about payments by themselves |
| `show_sync_problems` (`SHOW_SYNC_PROBLEMS`) | `true` | read-only: shows the last pull's outcome and problems on Collections (kept in memory, lost on restart) |
| `dashboard` (`DASHBOARD`) | `true` | read-only summary on `/accounting` for staff; each figure shows only to users who may see its source. `false` redirects to the first screen the user may use |
| `page_size` (`PAGE_SIZE`) | `50` | rows per page (10 to 200). Lists look through at most 2000 rows |
| `instance_url`, `instance_api_key`, `instance_timeout` (`INSTANCE_URL`, `INSTANCE_KEY`) | empty, 15s | no platform is invented; empty means "not connected" and Sync says so. https is required unless the URL is loopback; the key is required with the URL |
| `commission_bps_real`, `commission_bps_seam_ae`, `commission_bps_seam_web` (`COMMISSION_BPS_REAL`, `_SEAM_AE`, `_SEAM_WEB`) | `0` | the formula is outside the design; a made-up rate would book liabilities that do not exist. Rates are basis points (100 = 1%) of the amount collected, rounded half up to the cent; all zero accrues nothing. Enabled formula rules (see Formulas) take precedence over these rates; a richer formula can also go in code (`accounting.WithCalculator`) |
| `accountant_edit_formulas` (`ACCOUNTANT_EDIT_FORMULAS`) | `false` | formula rules decide what AEs are paid, so only the admin changes them. Admin staff and the accountant can view them and use the simulator. `true` lets the accountant create, change, reorder and delete rules |
| `formulas_oql` (`FORMULAS_OQL`) | `true` | allows the OQL method of a rule (see Formulas). `false` hides it, refuses to save or enable such rules and does not run stored ones. Only users who may edit formulas can write one |

### Platform API contract

With `instance_url` set, Admin calls (all with `Authorization: Bearer <key>`, redirects are
never followed):

    GET  {url}/seam-admin/v1/snapshot
         {"tenants":[{"external_ref","name","country","ae_ref","account","registration"}],
          "items":[{"external_ref","tenant_ref","period":"YYYY-MM","concept","description",
                    "amount":"123.45","state":"pending|collection_in_progress"}]}
    POST {url}/seam-admin/v1/items/{external_ref}/collected      (idempotent, any 2xx)

`ae_ref` is the id of a real AE in Admin, or `seam_ae` / `seam_web`. Amounts are USD decimal
strings. An item reported as `collected` fails the pull: Collected is decided in Admin only.

## Formulas (discounts and surcharges)

`/accounting/formulas` holds ordered rules in two situations: **commission** (what an AE earns
on a collected item) and **price adjustment** (a market's price from the base list price, for
example 20% off in Paraguay). Every rule has an effect and a method.

| Effect | Does |
|---|---|
| Set the amount | replaces the running amount |
| Surcharge | adds to it |
| Discount | subtracts from it (never below zero) |

| Method | The rule's figure is |
|---|---|
| Simple percentage | a percentage (two decimals) of the reference amount for *set*, of the running amount for a surcharge or discount |
| Fixed amount | an ad-hoc USD amount |
| OQL queries | the single value the last of one to five OQL queries returns, in USD cents |

Rules run top to bottom over a running amount (commission starts at 0, price at the base
price). A rule can be limited to an AE kind (commission), a country and a concept, can be
switched off without deleting it, and can stop the evaluation when it applies. The panel has a
simulator that applies the saved rules to sample values and shows each step; nothing is saved
or booked by it.

**Commission.** When at least one commission rule is enabled the rules decide and the
`commission_bps_*` rates are ignored; with none enabled the rates (or a calculator set in code)
apply as before. If a rule cannot be evaluated (store down, query error) the collection is
refused with a message rather than booking a guessed amount. The result is stored on the
commission line as before; changing or deleting rules never changes lines already booked.

**Price adjustment** is available as `Service.AdjustPrice` and in the simulator. Nothing in
the collection flow calls it: prices live in the platform, and the design makes Seam Admin
agnostic about them.

### OQL rules

Queries are separated by a line holding only `;`. The earlier queries are for working things
out: each result is available to the next as `:prev` and by position as `:q1`, `:q2`...
Values are passed only as named parameters, filled in as escaped literals, never by pasting
text: `:amount` (the reference), `:running`, `:country`, `:concept`, and for commissions
`:period`, `:tenant_id`, `:ae_id`, `:ae_kind`, `:item_id`. The last query must give one row
and one column; no rows is 0, a negative result is an error (use the discount effect).

Because the text is written by a person and runs on the module's database, it is checked when
saved and again each time it runs: a single `SELECT` (no `;` inside, comments, or
INSERT/UPDATE/DELETE/DROP/CREATE/ALTER/TRUNCATE/EXEC/MERGE/GRANT/REVOKE, no `@` or `NEXT VALUE`),
at most 2000 characters and 5 queries, 5 seconds per query, tables written plainly (no quotes or
brackets) and listed with `JOIN`, never commas. Only these tables may be read: `adm_item`,
`adm_tenant`, `adm_commission`, `adm_settlement`, `adm_close`, `adm_entry`, `adm_movement`,
`adm_account`, `adm_line_claim` (`accounting.OQLTables`). Not readable: AE profiles (payout
details), documents, company data, the platform record and the formulas themselves. What a
query returns is never copied into an error message.

## Roles and permissions

| Role | Can |
|---|---|
| `admin` | everything (`accounting.*`), including editing formulas |
| `admin_staff` | AEs (view, authorize), collections (view, sync, collect), commissions, settlements (view, pay), ledger (read-only), company |
| `accountant` | ledger, entries (create, reverse), monthly close, view of collections, commissions and settlements |
| `account_executive` | own portal, own profile, own settlements (create, view); nothing else |

Row-level rule: an AE sees only data attached to the AE linked to its login. Settlement
detail and document download answer "not found" for another AE's records. A user with the
`account_executive` role is registered as a real AE on first visit and cannot operate until
staff authorize it.

## Design mapping

- Item states: Pending, Collection in Progress, Collected. Collected is set by hand and, in
  one atomic write, books the collection and accrues the commission.
- Commission lines come from the formula rules (see Formulas), or from the configured rates
  when no rule is enabled, and are stored as results in USD.
- Settlement: the AE picks Collected lines (max 25) and uploads an invoice equal to the
  total, made out to the company legal name; staff upload the proof and mark it Paid.
  Claim records keyed by commission-line id make "paid once" a storage guarantee.
- Ledger: chart groups 1-5, postable leaves only, debit = credit, append-only with reversal,
  monthly close in order; foreign-currency document data is passive metadata.

## Remaining assumptions

1. The admin_staff / accountant split above is the default; `accountant_operate` and
   `staff_post_entries` move the two contested duties.
2. Staff cannot create AEs (they register themselves) and see only the last pull's
   problems, not a history of pulls.
3. The sidebar entry shows to any user whose role has accounting access; with `tenant_id`
   set, users of another tenant see it but get 403.

## Screens

- Irreversible actions ask first, in the app's own dialog: marking an item collected (on its
  own page, which shows what is being collected), authorizing an AE, creating a settlement,
  paying one, posting a manual entry, reversing an entry and closing a month.
- The manual entry form is a table with running debit and credit totals and a "balanced /
  difference" line; the settlement form keeps a count and total of the selected lines and
  pre-fills the invoice total. The server validates everything again.
- Ledger movements are laid out as side, account and amount; money is always `USD 1,234.50`
  and right-aligned; dates are dates, not timestamps.
- A result message removes itself from the address bar after it shows, so a refresh does not
  repeat it. A settlement's page lists its commission lines.
- Utility classes the precompiled stylesheet lacks live in `web/static/css/app.css`; a test
  (`TestAccountingClassesExist`) fails when a screen uses a class that exists in neither file.
- If a settlement cannot be created (lost race, store error) its draft and uploaded file are
  removed, so nothing is left behind.

## AE portal (what the AE sees)

Per the design (§6.4) the AE sees what it is due to receive, never what the customer pays:

- **My account with Seam**: the balance of the AE's own 2.1.x account and its movements, newest
  first, each labelled with its rubro (commission, settlement, reversal, manual), with earned,
  paid out and running balance. It reads only that account.
- **My commissions**: commission lines by month, tenant, concept and detail, and the settlement
  that paid each (or a dash).
- **Billable items**: period, tenant, concept and state only. The item amount is not shown.

An AE that Seam has not authorized has no account yet and sees none of these panels.
(`TestAccountingPortalHidesCustomerAmounts`, `...ShowsOwnAccountOnly`, `...AccountNeedsAuthorization`.)

### Tenants panel

The portal also lists **the AE's own tenants** (name, country, customer account, registration),
for the AE's id only; a request without an AE id is refused rather than listing everyone, and
an unauthorized AE sees no panel
(`TestAccountingPortalListsOwnTenantsOnly`, `...TenantsAbsentUntilAuthorized`).

## Documents on manual entries

A manual entry can carry an optional supporting document (receipt, invoice or other) in the
same form: file, number, date, and, as passive metadata, foreign currency, total, rate and
date. The file is stored with the entry in one step: `PostEntryWithDocument` validates the entry
and the open period first, saves the document, posts the entry, and removes the document if
posting fails. The entry description links to the document. These documents are marked
`for_entry` and are visible only to roles with ledger view, **never to AEs**.

## Demonstration data

`seeds/accounting/seed-accounting.seed` fills the module: five users (`admin_staff`,
`accountant` and three `account_executive`; the logins are in the file's header), five AEs,
seven tenants, 38 items from July to November 2026, the journal, commission lines, four
settlements, four monthly closes and five formula rules.

```bash
python3 seeds/seed.py load accounting --url http://localhost:9090
```

It loads alone or over any other seed, before or after the module has started: every record is
found by a field of its own (an account by its code, an AE by its slug or email, an entry by
its uid) and left alone if it is there. The file is written by `cmd/accounting-seedgen`
(`make seed-accounting`), which runs this package's Service over a store in memory, with a
fake platform and a clock moved through the five months. Change the generator, not the file.
The seed has the record of each document and not the file, so a seeded invoice or proof does
not open.

## Not verified

The xolu store was tested against a fake HTTP server, and the platform client against a
local test server. On 2026-11-18 the module was run in a browser over a live xolu 0.30.44
holding the demonstration data: its screens as back office, accountant and AE, and one
collection. The platform client has not been run against a platform.
