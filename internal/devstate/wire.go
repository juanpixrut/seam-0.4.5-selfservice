package devstate

import (
	"github.com/ha1tch/seam-ui/internal/i18n"
	"github.com/ha1tch/seam-ui/internal/schemaoverlay"
)

// Wire makes the Manager drive the lookup options of the translator and the
// schema resolver: whenever a tenant's effective state may have changed, its
// overlay switches are pushed into both. The hot-load flags are not applied
// here; they decide whether the background pollers run (Manager.AnyOn).
//
// Either argument may be nil.
func Wire(m *Manager, tr *i18n.Translator, res *schemaoverlay.Resolver) {
	m.OnChange(func(tenant int64) {
		v := m.Values(tenant)
		if tr != nil {
			tr.SetOptions(tenant, i18n.Options{
				DisableFilesystem: !v.OverlayFilesystem,
				DisableXolu:       !v.OverlayXolu,
				ShowKeys:          v.ShowI18NKeys,
			})
		}
		if res != nil {
			res.SetOptions(tenant, schemaoverlay.Options{
				DisableFilesystem: !v.OverlayFilesystem,
				DisableXolu:       !v.OverlayXolu,
			})
		}
	})
}
