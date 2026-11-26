package dashboardengine

import (
	"testing"
	"time"
)

// TestFormatNumber ports helpers_test.go's own TestFormatNumber exactly
// (same cases) -- the pre-migration internal/ui/dashboard.go had its own
// formatNumber, tested there; this package's own copy (widgets.go) is
// unexported and in a different package, so that coverage couldn't just
// be retargeted, it had to move with the function it tests.
func TestFormatNumber(t *testing.T) {
	cases := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{999999, "999.9K"},
		{1000000, "1M"},
		{2500000, "2.5M"},
	}
	for _, c := range cases {
		got := formatNumber(c.input)
		if got != c.want {
			t.Errorf("formatNumber(%d) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestCompileScalar(t *testing.T) {
	for _, wt := range []string{"stat_card", "total_card"} {
		got, err := compileScalar(wt, 7)
		if err != nil {
			t.Fatalf("compileScalar(%s): %v", wt, err)
		}
		field := "count"
		if wt == "total_card" {
			field = "total"
		}
		if compiledInt(got, field) != 7 {
			t.Errorf("compileScalar(%s) = %#v, want %s=7", wt, got, field)
		}
	}
}

func TestCompileRows_RecentList(t *testing.T) {
	rows := []map[string]any{
		{"id": int64(1), "title": "Temp exceeded", "asset_name": "Chiller 5"},
		{"id": int64(2), "title": "Low battery", "asset_name": "Sensor 12"},
	}
	got, err := compileRows("recent_list", rows)
	if err != nil {
		t.Fatalf("compileRows: %v", err)
	}
	compiledRows := compiledRows(got)
	if len(compiledRows) != 2 {
		t.Fatalf("compiledRows = %d rows, want 2", len(compiledRows))
	}
	if mapString(compiledRows[0], "title") != "Temp exceeded" {
		t.Errorf("row 0 title = %q, want %q", mapString(compiledRows[0], "title"), "Temp exceeded")
	}
	if mapString(compiledRows[1], "asset_name") != "Sensor 12" {
		t.Errorf("row 1 asset_name = %q, want %q", mapString(compiledRows[1], "asset_name"), "Sensor 12")
	}
}

func TestCompileRows_Empty(t *testing.T) {
	got, err := compileRows("recent_list", []map[string]any{})
	if err != nil {
		t.Fatalf("compileRows: %v", err)
	}
	if len(compiledRows(got)) != 0 {
		t.Errorf("compiledRows = %#v, want empty, not nil/missing", got["rows"])
	}
}

func TestGroupCountRowsToMaps(t *testing.T) {
	rows := []GroupCountRow{{Value: "active", Count: 5}, {Value: "lost", Count: 1}}
	maps := groupCountRowsToMaps(rows)
	if len(maps) != 2 || mapString(maps[0], "value") != "active" || compiledInt(maps[0], "count") != 5 {
		t.Errorf("groupCountRowsToMaps = %#v", maps)
	}
}

func TestUrgencyRowsToMaps(t *testing.T) {
	rows := []UrgencyRow{{EntityID: 3, EntityName: "Asset X", FieldName: "warranty_end", Date: "01/01/2027", DaysLeft: 12}}
	maps := urgencyRowsToMaps(rows)
	if len(maps) != 1 || mapString(maps[0], "entity_name") != "Asset X" || compiledInt(maps[0], "days_left") != 12 {
		t.Errorf("urgencyRowsToMaps = %#v", maps)
	}
}

func TestCompile_UnknownWidgetType(t *testing.T) {
	if _, err := compileScalar("not_a_real_type", 1); err == nil {
		t.Error("compileScalar with an unknown widget type = nil error, want an error")
	}
}

// -- cache.go --

func TestWidgetCache_HitAndExpiry(t *testing.T) {
	c := newWidgetCache(5 * time.Second)
	now := time.Now()
	c.now = func() time.Time { return now }

	c.set("k", map[string]any{"count": 1})
	if _, ok := c.get("k"); !ok {
		t.Fatal("expected a hit immediately after set")
	}

	now = now.Add(4 * time.Second)
	if _, ok := c.get("k"); !ok {
		t.Error("expected a hit before TTL elapses")
	}

	now = now.Add(2 * time.Second) // total 6s, past the 5s TTL
	if _, ok := c.get("k"); ok {
		t.Error("expected a miss after TTL elapses")
	}
}

func TestWidgetCache_DisabledWhenTTLZero(t *testing.T) {
	c := newWidgetCache(0)
	c.set("k", map[string]any{"count": 1})
	if _, ok := c.get("k"); ok {
		t.Error("a zero-TTL cache should never hit")
	}
}

func TestSourceCacheKey_VariesByCurrentUserID(t *testing.T) {
	// CurrentUserID carries json:"-" (it's not part of the schema), so
	// this must be checked explicitly -- json.Marshal alone would never
	// catch a regression here, since the field would silently vanish
	// from the marshaled key and two different users' scope: "self"
	// audit_log results would collide in the cache.
	base := EntitySource{Type: "audit_log", Resource: "profile", Scope: "self"}
	a := base
	a.CurrentUserID = 1
	b := base
	b.CurrentUserID = 2

	ka := sourceCacheKey("recent_list", a)
	kb := sourceCacheKey("recent_list", b)
	if ka == kb {
		t.Fatalf("two different users produced the same cache key: %q -- scope:self results would leak across users", ka)
	}
}

func TestSourceCacheKey_StableAndDistinct(t *testing.T) {
	a := EntitySource{Type: "entity", Entity: "assets", Filter: "status = 'open'"}
	b := EntitySource{Type: "entity", Entity: "assets", Filter: "status = 'open'"}
	c := EntitySource{Type: "entity", Entity: "assets", Filter: "status = 'closed'"}

	ka, kb, kc := sourceCacheKey("stat_card", a), sourceCacheKey("stat_card", b), sourceCacheKey("stat_card", c)
	if ka != kb {
		t.Errorf("identical sources produced different keys: %q vs %q", ka, kb)
	}
	if ka == kc {
		t.Errorf("different sources produced the same key: %q", ka)
	}
	if sourceCacheKey("total_card", a) == ka {
		t.Error("different widget types with the same source produced the same key")
	}
}
