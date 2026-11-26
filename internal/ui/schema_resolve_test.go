package ui

import (
	"context"
	"strings"
	"sync"
	"testing"

	mi "github.com/ha1tch/minty"

	"github.com/ha1tch/seam-ui/internal/auth"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

const (
	baseSchema     = `{"type":"object","properties":{"alpha":{"type":"string","title":"Alpha base","x-seam-input":"text"}}}`
	overrideSchema = `{"type":"object","properties":{"alpha":{"type":"string","title":"Alpha OVERRIDE","x-seam-input":"text"}}}`
)

func tenantCtx(tenant int64) context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{TenantID: tenant})
}

func withResolver(t *testing.T, r *schemaoverlay.Resolver) {
	t.Helper()
	SetSchemaResolver(r)
	t.Cleanup(func() { SetSchemaResolver(nil) })
}

func validator(b []byte) error { _, err := fe.ParseSchema(b); return err }

func TestResolveSchema_NoResolverIsTheEmbeddedSchema(t *testing.T) {
	SetSchemaResolver(nil)
	got := resolveSchema(context.Background(), "system", "user_edit", []byte(baseSchema))
	if string(got) != baseSchema {
		t.Errorf("without the Developer module the embedded schema must come back unchanged")
	}
}

func TestResolveSchema_OverrideAppliesPerTenantAndHonoursTheSwitch(t *testing.T) {
	r := schemaoverlay.New(validator)
	if err := r.ReloadFilesystem(map[schemaoverlay.Key][]byte{{Module: "system", Name: "user_edit"}: []byte(overrideSchema)}); err != nil {
		t.Fatal(err)
	}
	withResolver(t, r)
	base := []byte(baseSchema)
	if got := resolveSchema(tenantCtx(7), "system", "user_edit", base); string(got) != overrideSchema {
		t.Errorf("tenant 7 must see the override, got %s", got)
	}
	r.SetOptions(7, schemaoverlay.Options{DisableFilesystem: true})
	if got := resolveSchema(tenantCtx(7), "system", "user_edit", base); string(got) != baseSchema {
		t.Errorf("tenant 7 switched the filesystem layer off and must see the embedded schema")
	}
	if got := resolveSchema(tenantCtx(8), "system", "user_edit", base); string(got) != overrideSchema {
		t.Errorf("another tenant is unaffected by tenant 7's switch")
	}
	if got := resolveSchema(tenantCtx(7), "system", "user_create", base); string(got) != baseSchema {
		t.Errorf("a schema with no override is left alone")
	}
}

func TestResolveSchema_TheFormEngineRendersTheOverride(t *testing.T) {
	r := schemaoverlay.New(validator)
	if err := r.ReloadFilesystem(map[schemaoverlay.Key][]byte{{Module: "system", Name: "user_edit"}: []byte(overrideSchema)}); err != nil {
		t.Fatal(err)
	}
	withResolver(t, r)
	e := &fe.FormEngine{}
	render := func(b []byte) string {
		return mi.RenderToString(e.Render(context.Background(), b, nil, nil, fe.RenderContext{Locale: "en"}))
	}
	html := render(resolveSchema(tenantCtx(1), "system", "user_edit", []byte(baseSchema)))
	if !strings.Contains(html, "Alpha OVERRIDE") || strings.Contains(html, "Alpha base") {
		t.Errorf("the rendered form must come from the override:\n%s", html)
	}
}

// Adversarial: a half-saved or broken override never replaces a good one.
func TestResolveSchema_ABrokenReloadKeepsTheLastGoodOverride(t *testing.T) {
	r := schemaoverlay.New(validator)
	k := schemaoverlay.Key{Module: "system", Name: "user_edit"}
	if err := r.ReloadFilesystem(map[schemaoverlay.Key][]byte{k: []byte(overrideSchema)}); err != nil {
		t.Fatal(err)
	}
	withResolver(t, r)
	for name, bad := range map[string]string{"not json": `{"type":`, "not a schema": `[1,2,3]`, "empty": ``, "null": `null`} {
		if err := r.ReloadFilesystem(map[schemaoverlay.Key][]byte{k: []byte(bad)}); err == nil {
			t.Errorf("%s: the reload must be refused", name)
		}
		if got := resolveSchema(tenantCtx(1), "system", "user_edit", []byte(baseSchema)); string(got) != overrideSchema {
			t.Errorf("%s: the previous good override must stay in force, got %s", name, got)
		}
	}
}

// Adversarial: odd identities must not panic or leak another tenant's view.
func TestResolveSchema_HostileClaims(t *testing.T) {
	r := schemaoverlay.New(validator)
	if err := r.ReloadFilesystem(map[schemaoverlay.Key][]byte{{Module: "system", Name: "user_edit"}: []byte(overrideSchema)}); err != nil {
		t.Fatal(err)
	}
	withResolver(t, r)
	for _, ctx := range []context.Context{
		context.Background(), tenantCtx(0), tenantCtx(-1), tenantCtx(1 << 62),
		auth.ContextWithClaims(context.Background(), nil),
	} {
		_ = resolveSchema(ctx, "system", "user_edit", []byte(baseSchema)) // must not panic
	}
	if got := resolveSchema(tenantCtx(-5), "system", "user_edit", []byte(baseSchema)); string(got) != overrideSchema {
		t.Errorf("the filesystem layer is global: a negative tenant still sees it")
	}
}

// Adversarial: the resolver is swapped while pages resolve (run with -race).
func TestResolveSchema_InstallWhileResolvingIsSafe(t *testing.T) {
	r := schemaoverlay.New(validator)
	_ = r.ReloadFilesystem(map[schemaoverlay.Key][]byte{{Module: "system", Name: "user_edit"}: []byte(overrideSchema)})
	t.Cleanup(func() { SetSchemaResolver(nil) })
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				if (i+j)%2 == 0 {
					SetSchemaResolver(r)
				} else {
					SetSchemaResolver(nil)
				}
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				got := string(resolveSchema(tenantCtx(1), "system", "user_edit", []byte(baseSchema)))
				if got != baseSchema && got != overrideSchema {
					t.Errorf("a resolve returned neither schema: %s", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
