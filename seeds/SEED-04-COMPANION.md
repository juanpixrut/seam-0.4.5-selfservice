# Seed 04 — VendiBeria S.L.

## Companion Document: Vending Machines Vertical

**Vertical:** Operación de máquinas expendedoras (vending)
**Market:** Spain — Comunidad de Madrid and Castilla-La Mancha
**Date:** April 2026

---

## 1. Domain Overview

VendiBeria S.L. is a mid-sized vending operator based in Madrid. They own
and operate ~120 machines across corporate offices, hospitals, universities,
transport hubs, and industrial parks. Two daily service routes, three route
drivers (reponedores), one dedicated technician, one office administrator.

The company is a member of ANEDA (Asociación Nacional Española de
Distribuidores Automáticos) and holds the AQS quality certification.

### Why this vertical is structurally different

In Constructec, FrigoCold, and Extinfire the asset *does work* or *sits in a
location*. A vending machine does both — and it also **generates revenue
event-by-event**. Every dispensation is simultaneously an inventory event,
a financial transaction, and (for food machines) a food-safety-relevant
action. This makes the event stream richer and the alerting logic more
commercially driven than in any previous seed.

The channel/slot model is unique to vending. A single snack machine might
have 40 product selections across 6–10 shelves. The machine is the Seam
AMS asset; the channels are configuration, not sub-assets. The
original insight (Doc 10, Doc 23): "you don't know what product, you know
what channel — requires channel→product mapping by configuration." The
virtual counters feature (Doc 10/12) would eventually model this, but the
seed works without it by treating each machine as a single trackable unit
whose aggregate stock level is estimated from visit events.

---

## 2. The Spanish Vending Market

Spain has approximately 550,000–600,000 vending machines operated by
around 1,700 companies. The market is highly fragmented.

**Key operators:**

| Operator | Scale | Notes |
|---|---|---|
| Selecta | ~44,000 machines | Multinational, market leader |
| Alliance Vending / Arbitrade | Multi-regional | 30+ years, rebranding to Arbitrade |
| Delikia Fresh | ~10 brands under umbrella | Second by revenue |
| Easy Vending | Growing, health focus | Vending saludable pioneer |
| Olevending | Public vending specialist | Since 1988 |

**Machine sales:** ~19,000 new machines/year (pre-pandemic peak). Coffee
machines represent ~74% of new sales. Catalonia leads by volume (26% of
national sales).

**Revenue:** ~EUR 2 billion in operation revenue. Market projected to reach
USD 2.47 billion by 2033 at 4.5% CAGR.

**Key trends (2025–2026):**

- Contactless/cashless payment now expected on all new installations
- Telemetry adoption accelerating (Caslab MÁSTER is the leading Spanish
  platform)
- Fresh food and healthy options growing rapidly
- Age-verification technology emerging (Galicia's Ley 6/2025 restricts
  energy drink sales to minors)
- Hospital vending under regulatory pressure (draft Real Decreto on
  healthy food in hospitals)
- Micromarkets / tiendas 24h as new format competing with traditional
  vending

---

## 3. Actors and Their Concerns

### 3.1 The Operator (empresa operadora) — our tenant

VendiBeria owns or leases the machines, manages routes, stocks products,
handles technical service. They are the Seam AMS tenant.

**Their daily concerns:**

- Route efficiency: which machines need restocking today?
- Stock visibility: what's running low, what's about to expire?
- Revenue tracking: which locations are performing, which are declining?
- Technical uptime: which machines are down, and for how long?
- Compliance: are refrigerated machines in temperature range? Are cleaning
  schedules current?

### 3.2 The Client (empresa cliente)

The office, hospital, university, or station that hosts the machine. They
do not own the machine. The relationship is contractual — sometimes the
operator pays a "canon" (commission) to the client for the location,
sometimes the client pays the operator for the service.

**Modelled as:** custodio. The client has custody of the location; the
operator retains ownership (calidad_tenencia: Propiedad) or the machine
is leased (calidad_tenencia: Alquiler).

### 3.3 The Route Driver (reponedor)

Physically visits machines on a daily/weekly route. Restocks products,
collects cash from coin mechanisms, performs basic cleaning, notes faults.
In a QR-based Seam AMS deployment, the route driver scans the machine's
QR at arrival and departure, generating visit events with timestamp and
GPS.

**Modelled as:** event source (trigger_type: qr_scan). The driver is not
a custodian — they are the human sensor.

### 3.4 The Technician (técnico SAT)

Handles faults: jammed spirals, coin mechanism failures, refrigeration
issues, payment terminal malfunctions. Separate role from route driver in
larger operations. May work on multiple operators' machines (some SAT
companies are independent).

**Modelled as:** event source (trigger_type: api_call or qr_scan for
service events).

### 3.5 The End Consumer

Mostly invisible to the asset management system. However, QR-based
problem reporting (Doc 10, section 4) would make them a human sensor
generating incident events. The seed includes a public-facing QR concept
but does not require it for MVP.

---

## 4. Regulatory Environment

Lighter than fire extinguishers (no DNB equivalent) but non-trivial:

**Food safety — APPCC (Análisis de Peligros y Puntos de Control Crítico):**
The Spanish implementation of HACCP. Any machine dispensing food must
comply. Temperature monitoring is mandatory for refrigerated machines.
Operators must maintain temperature logs, cleaning records, and product
traceability.

**Ley 6/2025 (Galicia):** Restricts sale of energy drinks to minors via
vending machines. Other autonomous communities are expected to follow.
Requires age-verification technology.

**Draft Real Decreto on hospital vending:** Under development. Would
restrict the types of products that can be sold in hospital vending
machines. ANEDA is actively lobbying against overly restrictive terms.

**CE marking and electrical safety:** Machines must comply with EU safety
directives. Relevant for older machines that may need to be retired.

**Payment regulation:** PSD2 compliance for cashless transactions. PCI DSS
for card data handling (typically handled by the payment terminal
provider, not the operator).

### What this means for the seed

The regulatory dimension is captured through:

- Temperature monitoring alerts on refrigerated machines
- Cleaning schedule tracking (alertas: limpieza vencida)
- Product expiry alerts (alertas: producto próximo a caducar)
- APPCC compliance as a prop_definition (status field)
- No need for a dedicated regulatory FSM (unlike Extinfire's DNB
  habilitación)

---

## 5. Data Model Decisions

### 5.1 Machine types vs. product types

The seed models **machine types** as asset_types, not product types. A
"Máquina de Snacks" is an asset type. The products inside it (crisps,
chocolate, nuts) are configuration — they change weekly and are not
individually tracked at the Seam AMS level. This is a deliberate
decision aligned with the product architecture: "you don't know what product,
you know what channel."

When virtual counters (Doc 10) are implemented, each channel within a
machine could become a counter with FIFO partitioning (Doc 12). The seed
structure supports this future extension without requiring it.

### 5.2 Payment terminals as separable assets

Payment terminals (cashless readers, NFC devices) are modelled as a
separate asset type rather than embedded in the machine definition. This
reflects operational reality: terminals fail independently, get firmware
updates independently, can be swapped between machines, and are sometimes
owned by a third-party payment service provider (not the operator).

A terminal's ubicacion matches the machine it's currently installed on.
When swapped, the terminal's location changes but the machine's doesn't.

### 5.3 Combo machines

Combo machines (snack + cold drink in a single unit) are their own asset
type, not two separate assets. In the physical world they are one unit
with one serial number, one power connection, and one service visit. The
dual temperature zone (ambient + refrigerated) is captured in the state
template.

### 5.4 Micromarkets

Micromarkets are a distinct asset type. They're not vending machines in
the traditional sense — they're open-seam self-service points with
camera/AI monitoring and a checkout kiosk. But from Seam AMS's
perspective, they're a managed asset at a client location with
maintenance, restocking, and revenue tracking needs. Including them in
the seed signals that the platform handles the "next generation" of
unattended retail alongside traditional vending.

### 5.5 Custody model

The operator (VendiBeria) owns most machines. The custodio is the
**client** at whose premises the machine is installed. A few machines are
on leasing from the manufacturer or distributor (calidad_tenencia:
Alquiler). Machines in the warehouse for repair or staging have
VendiBeria as custodian.

This inverts the Extinfire model: there, the client owns the
extinguisher and Extinfire provides service. Here, VendiBeria owns the
machine and the client provides the location.

### 5.6 Route-based operations

The centros_costo reflect routes, not maintenance categories. "Ruta
Madrid Centro" and "Ruta Madrid Sur-Toledo" are cost centres because
route efficiency is the primary operational lever. The seed maps each
machine to its route via centro_costo_id.

---

## 6. Brands and Models

All brands are real and verified in the Spanish market:

| Code | Brand | Origin | Speciality |
|---|---|---|---|
| AZK | Azkoyen | Peralta, Navarra, Spain | Coffee, tabaco — Spanish market leader |
| JOF | Jofemar | Peralta, Navarra, Spain | Snacks, drinks, coffee |
| NEC | Necta (Evoca) | Valbrembo, Italy | Coffee, snacks — highest revenue in Spain |
| BIA | Bianchi | Bergamo, Italy | Coffee — LEI 700 series |
| SDV | SandenVendo | US/Japan | Snacks, cold drinks — G-Snack, G-Drink |
| SAE | Saeco (Evoca) | Gaggio Montano, Italy | Coffee — compact, Phedra EVO |
| CSL | Caslab | Spain | Telemetry (MÁSTER device), not a machine brand |

### Models used in seed

| Brand | Model | Type | Notes |
|---|---|---|---|
| Azkoyen | Vitro S5 | Coffee | Superautomatic, touchscreen |
| Azkoyen | Vitro M | Coffee | Compact, for smaller locations |
| Necta | Krea Touch | Coffee | Premium, glass front |
| Necta | Melodia | Snacks | Mid-range, 8 shelves |
| Bianchi | LEI 700 Plus | Coffee | Fresh-ground, customisable |
| Jofemar | Vision EVO | Snacks | Large capacity, 10 shelves |
| Jofemar | J Series | Cold drinks | Refrigerated, can/bottle |
| Jofemar | Vision Combo | Combo | Snack + cold drink combined |
| SandenVendo | G-Snack | Snacks | Robust, easy maintenance |
| SandenVendo | G-Drink DM8 | Cold drinks | 8-selection, glass front |
| Saeco | Phedra EVO | Coffee | Compact, office segment |

---

## 7. FSM Design

### 7.1 Estado Operativo — Máquina Vending

Six states reflecting the operational lifecycle of a machine in the field:

```
operativa ──→ stock_bajo ──→ sin_stock
    │              │
    │              └──→ operativa (after restock)
    │
    ├──→ alarma_temperatura (refrigerated only)
    │         └──→ fuera_de_servicio
    │
    ├──→ fuera_de_servicio
    │         └──→ en_reparación
    │                   └──→ operativa
    │
    └──→ fuera_de_servicio (direct, e.g. vandalism)
```

### 7.2 Estado Ruta — Visita de Reposición

Three states for visit tracking:

```
pendiente ──→ en_curso ──→ completada ──→ pendiente (next cycle)
```

Triggered by QR scan at arrival (pendiente → en_curso) and departure
(en_curso → completada).

### 7.3 Ciclo de Vida — Máquina

Long-term lifecycle:

```
en_servicio ──→ mantenimiento_mayor ──→ en_servicio
      │
      └──→ retirada ──→ dada_de_baja
```

---

## 8. Edge Cases the Seed Should Exercise

| # | Edge case | How modelled |
|---|---|---|
| 1 | Old machine without telemetry | QR-only connectivity, no sensor bindings |
| 2 | New machine with Caslab MÁSTER | Gateway connectivity, telemetry sensor bound |
| 3 | Machine in workshop for repair | status: maintenance, location: ALMACEN-VBI |
| 4 | Machine vandalised/stolen | status: lost or inactive, motivo_baja if retired |
| 5 | Combo machine (dual zone) | Separate asset type, dual temperature in state |
| 6 | Fresh food with expiry concern | Active alert: producto próximo a caducar |
| 7 | Machine on leasing | calidad_tenencia: Alquiler, marca as lessor |
| 8 | Payment terminal swapped | Terminal asset at different location than usual |
| 9 | Micromarket (new format) | Distinct asset type, camera/AI in identity |
| 10 | Machine with very high revenue | Service config with tighter visit schedule |
| 11 | Machine at hospital (regulatory) | APPCC status tracked, cleaning schedule strict |
| 12 | Machine at university (seasonal) | Lower criticality, reduced summer schedule |
| 13 | Recently purchased (warranty) | fin_garantia set, newer manufacture year |
| 14 | Machine approaching depreciation | Old purchase date, no motivo_baja yet |
| 15 | EUR and USD pricing | One imported machine priced in USD |

---

## 9. Relationship to Design Documents

| Document | Relevance to Seed 04 |
|---|---|
| Doc 10 (Notas de Diseño v2) | Virtual counters concept originated from vending. Seed structure supports future slot-level tracking. |
| Doc 11 (Análisis de Verticales) | Vending machines listed as Tier 1 vertical (visit tracking) and implicitly Tier 2 (with telemetry). |
| Doc 12 (Gestión de Lotes) | FIFO dispensing model for fresh food machines. Seed's "Máquina de Comida Fresca" type is the future carrier for partitioned counters. |
| Doc 12-A (Validación Particiones) | Validates that vending is one of the 25 verticals where the partition model applies. |
| Doc 14 (Vertical Readiness) | Confirms vending visit tracking is Tier 1 (no code changes). Telemetry-based tracking is Tier 2. |
| Doc 23 (Business Development) | A prior vending case in Spain with Arduino sensors. Origin of the events-with-counters model. Validates the approach. |

---

## 10. Volume and Distribution Target

**Total assets:** ~120

| Asset type | Count | Notes |
|---|---|---|
| Máquina de Café | 40 | Dominant type, mirrors Spanish market (~74%) |
| Máquina de Snacks | 20 | Second most common |
| Máquina de Bebidas Frías | 15 | Refrigerated, seasonal peak |
| Máquina Combo | 8 | Smaller locations |
| Máquina de Comida Fresca | 5 | Growing segment, highest food safety |
| Terminal de Pago (separable) | 15 | Bound to machines but tracked independently |
| Micromarket | 3 | New format, premium locations |
| Workshop/retired | 4–5 | In repair, staging, or decommissioned |

**By client site:** 11 sites. Largest (Telefónica Distrito C) has 15–20
machines. Smallest (IES Cervantes) has 3–4. Hospital sites have stricter
cleaning/temperature requirements. University sites have seasonal
variation.

**By brand:** All 6 machine brands appear. Azkoyen and Necta dominate
coffee. Jofemar and SandenVendo dominate snacks/drinks. Saeco appears
in compact office installations. Bianchi for premium coffee.

**By manufacture year:** Span 2014–2025 to exercise depreciation and
lifecycle. A few 2014–2016 units approaching 10-year replacement cycle.

**By connectivity:** ~30% with Caslab telemetry (newer machines), ~70%
QR-only (older fleet). The telemetry subset generates temperature
readings, sales data, and payment terminal heartbeats. The QR-only
subset relies entirely on route driver visit events.

---

## 11. Prices

Spanish market reference prices (EUR, 2025):

| Type | New price range | Used/refurb |
|---|---|---|
| Coffee (basic) | 3,000–6,000 | 1,500–3,000 |
| Coffee (premium) | 6,000–12,000 | 3,000–6,000 |
| Snacks | 4,000–8,000 | 2,000–4,000 |
| Cold drinks | 3,500–7,000 | 1,800–3,500 |
| Combo | 6,000–12,000 | 3,000–6,000 |
| Fresh food (refrigerated) | 8,000–15,000 | 4,000–8,000 |
| Micromarket (full setup) | 15,000–30,000 | n/a |
| Payment terminal (cashless) | 300–800 | 150–400 |

Renting/leasing: EUR 95–539/month depending on type (Olevending published
rates). VendiBeria uses a mix of owned and leased machines.
