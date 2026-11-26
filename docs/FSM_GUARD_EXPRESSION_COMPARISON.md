# FSM Guard Expression Language Comparison

**Decision:** T-SQL via `tsqlparser` (settled — see `FSM_DESIGN.md` §5.3)

---

## Pattern comparison

| Pattern | CEL | ual compute block | T-SQL (tsqlparser) |
|---|---|---|---|
| Simple numeric comparison | `value.temperature > 5.0` | `var t = self.temperature`<br>`return t > 5.0` | `value.temperature > 5.0` |
| String equality | `value.door_state == "open"` | ✗ no string type | `value.door_state = 'open'` |
| AND | `value.temperature <= 8.0 && value.temperature > 5.0` | `var t = self.temperature`<br>`return t <= 8.0 && t > 5.0` | `value.temperature <= 8.0 AND value.temperature > 5.0` |
| OR | `a < min \|\| a > max` | `return a < min \|\| a > max` | `a < min OR a > max` |
| Null check | `value.next_due_date != null` | ✗ no null type | `value.next_due_date IS NOT NULL` |
| Set membership (literal) | `value.damage_type in ["corrosion", "impact"]` | ✗ no IN | `value.damage_type IN ('corrosion', 'impact')` |
| Set membership (dynamic) | `value.geofence_id in asset.permitted_geofence_ids` | ✗ no IN | `value.geofence_id NOT IN (SELECT v FROM asset.permitted_geofence_ids)` |
| Arithmetic threshold | `value.weight_kg < asset.min_weight_kg * 0.8` | `return self.weight_kg < self.min_weight_kg * 0.8` | `value.weight_kg < asset.min_weight_kg * 0.8` |
| Date arithmetic | `timestamp(asset.last_inspection_date).timeSince() >= duration("8760h")` | ✗ no date type | `calendar.days_since(asset.last_inspection_date) >= 365` |
| Date comparison | `timestamp(value.cert_expiry) > now()` | ✗ no date type | `value.cert_expiry > calendar.today` |
| Duration in state | `duration_in_state > duration("30m")` | `return self.duration_in_state > 1800` | `duration_in_state > 1800` |
| Range | `x >= 2.0 && x <= 5.0` | `return x >= 2.0 && x <= 5.0` | `value.temperature BETWEEN 2.0 AND 5.0` |
| Asset-only compound | `asset.age_days >= asset.min_age && asset.weight_kg >= asset.min_weight` | `return self.age_days >= self.min_age`<br>`&& self.weight_kg >= self.min_weight` | `asset.age_days >= asset.breed_min_slaughter_age_days AND asset.weight_kg >= asset.breed_min_slaughter_weight_kg` |
| Intra-event comparison | `value.gross >= value.declared * 0.95` | `return self.gross >= self.declared * 0.95` | `value.gross_weight_kg >= value.manifest_declared_weight_kg * 0.95` |
| Collection aggregate | `value.axle_weights.exists(w, w > asset.max_axle_weight_kg)` | `var i = 0`<br>`while i < len {`<br>`  if self[i] > max { return true }`<br>`  i = i + 1`<br>`}`<br>`return false` | `EXISTS (SELECT 1 FROM value.axle_weights w WHERE w > asset.max_axle_weight_kg)` |
| Calendar day | `timestamp(now()).getDayOfWeek("UTC") != 0` | ✗ no date type | `calendar.day_of_week != 0` |
| Inventory check | `asset.can_count > 0` | `return self.can_count > 0` | `asset.can_count > 0` |
| Sequence counter | `event.consecutive_count >= 3` | `return self.consecutive_count >= 3` | `event.consecutive_count >= 3` |

---

## Coverage summary

| Capability | CEL | ual compute | T-SQL (tsqlparser) |
|---|---|---|---|
| Numeric / boolean / arithmetic | ✓ | ✓ | ✓ |
| Strings | ✓ | ✗ | ✓ |
| Null | ✓ | ✗ | ✓ |
| IN / set membership | ✓ | ✗ | ✓ |
| BETWEEN / range | via `&&` | via `&&` | ✓ native |
| Dates | ✓ verbose | ✗ | ✓ via `calendar.*` |
| Collection aggregate | ✓ cleanest | ✓ via loop | ✓ via EXISTS |
| Already in project | ✗ | ✗ (in `main`) | ✓ |
| Go version requirement | 1.23+ | 1.22 | 1.22 |

---

## Note on ual compute blocks for actions

ual compute blocks are not suited for guards due to the string/null/IN gaps.
However, they are a strong candidate for **transition actions**, where
operations are predominantly numeric mutations on asset fields
(`asset.increment(can_count, -1)`, computing deadlines, threshold checks).
This requires extracting `cmd/iual/compute_compile.go` to a `pkg/compute/`
package — a conversation to have.
