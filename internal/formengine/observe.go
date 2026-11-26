package formengine

import (
	"context"
	"sync/atomic"
)

// RenderObserver is told, once per Render, which definition was rendered and
// under which i18n identity (the module and form its label keys are built
// from, FORM_ENGINE_SPEC section 4.1), after the precedence between the
// RenderContext and the definition's own x-seam-form is resolved.
type RenderObserver func(ctx context.Context, schema []byte, module, form string)

var renderObserver atomic.Pointer[RenderObserver]

// SetRenderObserver installs the observer, or removes it with nil. It is
// process-wide and meant for the Developer module, which uses it to name the
// translation keys a form's page really asks for; with none installed a
// render pays one atomic load.
func SetRenderObserver(o RenderObserver) {
	if o == nil {
		renderObserver.Store(nil)
		return
	}
	renderObserver.Store(&o)
}
