package ui

import (
	"context"
	"fmt"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	xolu "github.com/ha1tch/xolu/pkg/client"
)

// AssetSourceResolver implements formengine.SourceResolver for the
// Asset schema's x-seam-source fields, backed by the real xolu OQL
// queries the static form already uses. Every query and label format
// below was read directly out of internal/ui/masters_routes.go and
// asset_detail.go before being copied here, not reconstructed from
// FORM_ENGINE_SPEC.md §5's table alone (that table turned out accurate
// for every one of these nine sources, confirmed by checking each
// individually, but two other spec tables have been wrong before this
// session -- checking beats trusting).
type AssetSourceResolver struct {
	Client *xolu.Client
}

// assetSourceQuery pairs one x-seam-source key with the OQL query that
// resolves it and a function that turns one result row into an
// Option's label. Value is always the row's "id", stringified --
// every source here is an integer-keyed xolu entity.
type assetSourceQuery struct {
	oql   string
	label func(row map[string]any) string
}

var assetSourceQueries = map[string]assetSourceQuery{
	// entity:marcas -- codigo + " — " + nombre, confirmed against
	// asset_detail.go's marca_id option-building closure directly.
	"entity:brands": {
		oql: "SELECT * FROM brands ORDER BY name",
		label: func(row map[string]any) string {
			return toString(row["code"]) + " — " + toString(row["name"])
		},
	},
	"entity:models": {
		// Fixed 2026-08-11 -- was still querying marca_id/nombre,
		// which stopped existing on the models table once T-80
		// (Modelo's own rename) shipped; this specific query was
		// missed at the time since it lives in a different file from
		// where that rename's other work happened. Confirmed broken
		// directly: the asset edit form's Model dropdown was
		// rendering every option with an empty label. brand_id isn't
		// actually used by the label closure below, but selected here
		// anyway in case a future caller needs it, matching this
		// map's own entity:brands entry style.
		oql: "SELECT id, brand_id, name FROM models ORDER BY name",
		label: func(row map[string]any) string {
			return toString(row["name"])
		},
	},
	"entity:regions": {
		oql: "SELECT * FROM regions ORDER BY description",
		label: func(row map[string]any) string {
			return toString(row["description"])
		},
	},
	"entity:locations": {
		oql: "SELECT id, short_name, region_id FROM locations ORDER BY short_name",
		label: func(row map[string]any) string {
			return toString(row["short_name"])
		},
	},
	// WHERE active = true is load-bearing here, not incidental --
	// loadCustodiosSimple only lists active custodians, and this
	// resolver must match that or the new form would offer choices the
	// old one never did.
	"entity:custodians": {
		oql: "SELECT id, legal_name, subject_type FROM custodians WHERE active = true ORDER BY legal_name",
		label: func(row map[string]any) string {
			return toString(row["legal_name"])
		},
	},
	"entity:currencies": {
		oql: "SELECT id, iso_code, symbol FROM currencies ORDER BY iso_code",
		label: func(row map[string]any) string {
			return toString(row["iso_code"]) + " " + toString(row["symbol"])
		},
	},
	// The x-seam-source key and URL path spell this "centros-costo"
	// (hyphen), but the actual xolu table is "cost_centers"
	// (underscore) -- confirmed directly from loadCentrosCosto's own
	// query, not assumed from the source key's own spelling.
	"entity:cost-centers": {
		oql: "SELECT id, accounting_code, name FROM cost_centers ORDER BY name",
		label: func(row map[string]any) string {
			return toString(row["accounting_code"]) + " — " + toString(row["name"])
		},
	},
	// type/criticality here are entity:alert-types' OWN fields (T-40,
	// 2026-08-02), unrelated to the Asset schema's own type_id/criticality
	// -- confirmed by reading loadAlertas' query, not assumed from the
	// similar naming. Values remain Spanish in storage (Mantenimiento/
	// Alta/...) via alertaEnumValueToXolu -- T-23's separate, not-yet-
	// fixed concern -- but the field names themselves are English now.
	"entity:alert-types": {
		oql: "SELECT * FROM alert_types ORDER BY type, criticality",
		label: func(row map[string]any) string {
			return toString(row["type"]) + " (" + toString(row["criticality"]) + ")"
		},
	},
	"entity:asset-types": {
		oql: "SELECT id, name FROM asset_types ORDER BY name",
		label: func(row map[string]any) string {
			return toString(row["name"])
		},
	},
}

// Resolve implements formengine.SourceResolver.
func (r *AssetSourceResolver) Resolve(ctx context.Context, source string) ([]fe.Option, error) {
	q, ok := assetSourceQueries[source]
	if !ok {
		return nil, fmt.Errorf("asset source resolver: unrecognised source %q", source)
	}
	result, err := r.Client.OQL(ctx, q.oql)
	if err != nil {
		if entityNotYetCreated(err) {
			// A genuinely fresh tenant has no rows -- and therefore no
			// entity -- for this master table yet (xolu creates an entity
			// implicitly on first write; there's no separate schema-
			// registration step). Matches the static form's own
			// loadMarcas/loadAssetTypes/etc. behaviour for the identical
			// case: an empty, functional dropdown, not a broken form.
			// Confirmed live (T-28 evidence work): before this fix, every
			// FK-sourced field on a fresh instance rendered
			// "XOLU-QL004: entity does not exist" in place of the widget,
			// including type_id, which is required -- making the form
			// genuinely unsubmittable on the very first session of a new
			// deployment.
			//
			// Deliberately narrow, not a blanket swallow of every OQL
			// error the way the static form's loaders do -- a real
			// failure (network error, a genuinely malformed query, a
			// server-side problem) still propagates below, visible to
			// the operator, rather than silently degrading to an empty
			// dropdown with no indication anything's wrong.
			return []fe.Option{}, nil
		}
		return nil, fmt.Errorf("asset source resolver: querying %q: %w", source, err)
	}
	options := make([]fe.Option, 0, len(result.Data))
	for _, row := range result.Data {
		// toString only handles the string case (returns "" for
		// anything else) -- "id" comes back as a JSON number
		// (float64), so it must go through toInt64 first. Verified by
		// reading toString's actual implementation before using it
		// here, not assumed to handle numbers because its name
		// suggests it might.
		options = append(options, fe.Option{
			Value: fmt.Sprintf("%d", toInt64(row["id"])),
			Label: q.label(row),
		})
	}
	return options, nil
}

// entityNotYetCreated reports whether err is xolu's "entity does not
// exist" query-validation failure specifically -- not every error
// under the same XOLU-QL004 code, which is a general "query failed"
// bucket (pkg/errors/errors.go) covering multiple distinct causes.
// The code alone is too broad to treat as safe-to-ignore; the message
// pattern is generated from exactly one place in xolu itself
// (pkg/oql/validator.go's `entity '%s' does not exist`), confirmed by
// reading that source directly, not assumed from the message text
// alone.
func entityNotYetCreated(err error) bool { return auth.EntityNotYetCreated(err) }
