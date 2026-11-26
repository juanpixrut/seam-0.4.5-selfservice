package formengine

import (
	"context"
	"strings"
	"testing"

	mi "github.com/ha1tch/minty"
	asset "github.com/ha1tch/seam-ui/internal/formengine/schemas/asset"
)

func TestZZZ_DoesRenderEmitAFormTag(t *testing.T) {
	schema := asset.Schema()
	e := &FormEngine{Sources: StaticSourceResolver{}}
	html := mi.RenderToString(e.Render(context.Background(), schema, nil, nil, RenderContext{}))
	t.Logf("contains <form: %v", strings.Contains(html, "<form"))
	t.Logf("contains </form>: %v", strings.Contains(html, "</form>"))
	idx := strings.Index(html, "<form")
	if idx >= 0 {
		end := idx + 200
		if end > len(html) {
			end = len(html)
		}
		t.Logf("form tag context: %s", html[idx:end])
	}
}
