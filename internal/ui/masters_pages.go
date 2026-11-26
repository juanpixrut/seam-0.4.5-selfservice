package ui

// masters_pages.go — minty page rendering for all Wave 0 master table entities.

import (
	"fmt"

	mi "github.com/ha1tch/minty"
)

// ── Shared helpers ────────────────────────────────────────────────────────────

// masterFormInput renders a labelled text input for master forms.
func masterFormInput(b *mi.Builder, label, name, value, inputType string, required bool) mi.Node {
	return masterFormInputErr(b, label, name, value, inputType, required, "")
}

// masterFormInputErr is masterFormInput with a per-field error message.
func masterFormInputErr(b *mi.Builder, label, name, value, inputType string, required bool, errMsg string) mi.Node {
	reqMark := ""
	var inputNode mi.Node
	const baseCls = "mt-1 w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:border-indigo-500"
	borderCls := "border-gray-300 dark:border-gray-600 focus:ring-indigo-500"
	if errMsg != "" {
		borderCls = "border-red-500 dark:border-red-500 focus:ring-red-500"
	}
	cls := baseCls + " " + borderCls
	if required {
		reqMark = " *"
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", value), mi.Required())
	} else {
		inputNode = b.Input(mi.Type(inputType), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", value))
	}
	nodes := []interface{}{
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), label+reqMark),
		inputNode,
	}
	if errMsg != "" {
		nodes = append(nodes, b.P(mi.Class("mt-1 text-xs text-red-600 dark:text-red-400"), errMsg))
	}
	return b.Div(append([]interface{}{mi.Class("mb-4")}, nodes...)...)
}

// masterFormSelect renders a labelled select for master forms.
// masterFormSelect renders a labeled <select>. Falls back to the
// literal Spanish placeholder when t is nil, not the raw key --
// shared by Custodio, Alerta, and PropAssoc's forms, not all updated
// in the same T-18 pass.
func masterFormSelect(b *mi.Builder, label, name, selected string, required bool, options []struct{ V, L string }, t func(key string, args ...any) string) mi.Node {
	reqMark := ""
	if required {
		reqMark = " *"
	}
	placeholder := "— Select —"
	if t != nil {
		placeholder = t("masters.select_placeholder")
	}
	opts := []interface{}{
		mi.Name(name),
		mi.ID(name),
		mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"),
	}
	if required {
		opts = append(opts, mi.Required())
	}
	opts = append(opts, b.Option(mi.Value(""), placeholder))
	for _, o := range options {
		if o.V == selected {
			opts = append(opts, b.Option(mi.Value(o.V), mi.Selected(), o.L))
		} else {
			opts = append(opts, b.Option(mi.Value(o.V), o.L))
		}
	}
	return b.Div(mi.Class("mb-4"),
		b.Label(mi.For(name),
			mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"),
			label+reqMark,
		),
		b.Select(opts...),
	)
}

// masterFormError renders an error banner inside a form.
func masterFormError(b *mi.Builder, msg string) mi.Node {
	if msg == "" {
		return b.Span()
	}
	return b.Div(mi.Class("mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-400"),
		msg,
	)
}

// masterFormButtons renders Save / Cancel buttons for master forms.
// Falls back to the literal Spanish text when t is nil, not the raw
// key -- shared by every master entity's form, most of which don't
// pass t yet (T-18 staged rollout); the literal preserves their exact
// prior behaviour untouched.
func masterFormButtons(b *mi.Builder, cancelHref string, t func(key string, args ...any) string) mi.Node {
	save, cancel := "Save", "Cancel"
	if t != nil {
		save, cancel = t("action.save"), t("action.cancel")
	}
	return b.Div(mi.Class("flex gap-3 pt-4 justify-end"),
		b.Button(
			mi.Type("submit"),
			mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"),
			save,
		),
		b.A(
			mi.Href(cancelHref),
			mi.Class("px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 text-sm font-medium"),
			cancel,
		),
	)
}

// masterPartialFormWrapper wraps form fields for SeamModal2 partial rendering.
// The form uses hx-post targeting #seam-modal2-body. Action buttons are
// emitted in [data-modal-footer] so SeamModal2's JS moves them to the
// fixed footer outside the scrollable body.
func masterPartialFormWrapper(b *mi.Builder, formID, hxPost string, inner []interface{}, t func(key string, args ...any) string) mi.Node {
	save, cancel := "Save", "Cancel"
	if t != nil {
		save, cancel = t("action.save"), t("action.cancel")
	}
	formAttrs := []interface{}{
		mi.ID(formID),
		mi.Class("space-y-4 pb-2"),
		mi.Attr("hx-post", hxPost),
		mi.Attr("hx-target", "#seam-modal2-body"),
		mi.Attr("hx-swap", "innerHTML"),
	}
	formAttrs = append(formAttrs, inner...)
	return b.Div(
		b.Form(formAttrs...),
		b.Div(mi.Attr("data-modal-footer", ""),
			b.Div(mi.Class("flex gap-3 justify-end w-full"),
				b.Button(mi.Type("button"),
					mi.Class("px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 font-medium text-sm"),
					mi.Attr("onclick", "SeamModal2.close()"),
					cancel,
				),
				b.Button(mi.Type("submit"),
					mi.Attr("form", formID),
					mi.Class("px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 font-medium text-sm"),
					save,
				),
			),
		),
	)
}

// masterFormCard wraps form content in a card container.
func masterFormCard(b *mi.Builder, children ...interface{}) mi.Node {
	return b.Div(
		mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
		b.Form(append([]interface{}{mi.Method("post"), mi.Class("space-y-1")}, children...)...),
	)
}

// ── Región ────────────────────────────────────────────────────────────────────

func RegionesListPage(data RegionesListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.region.edit")
	rows := make([]mi.H, len(data.Regions))
	for i, r := range data.Regions {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", r.ID)),
			ListCellLink(r.Description, fmt.Sprintf("/regions/%d", r.ID)),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/regions/%d/edit", r.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "600px",
				DeleteURL:  fmt.Sprintf("/regions/%d/delete", r.ID),
				DeleteName: r.Description,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.region.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/regions/new", CreateLabel: t("masters.region.new"),
			CreateTargetModal2: true, ModalWidth: "600px",
			MaxWidth: "max-w-2xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.region.field.description")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "public", EmptyTitle: t("masters.region.empty_title"),
			EmptyMessage: t("masters.region.empty_message"),
		}, rows),
		nil,
	)
}

func RegionFormPage(data RegionFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/regions"
	if data.IsEdit {
		action = fmt.Sprintf("/regions/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.region.field.description"), "description", data.Description, "text", true, fieldError(data.Errors, "description")),
		}
		if data.Fields != nil {
			inner = append(inner[:2:2], data.Fields(b))
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.region.edit"), false: t("masters.region.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, append(inner, masterFormButtons(b, "/regions", t))...)...),
			),
		)
	}
}

// ── Moneda ────────────────────────────────────────────────────────────────────

func MonedasListPage(data MonedasListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.currency.edit")
	rows := make([]mi.H, len(data.Currencies))
	for i, m := range data.Currencies {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", m.ID)),
			ListCellLink(m.IsoCode, fmt.Sprintf("/currencies/%d", m.ID)),
			ListCell(m.Symbol),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/currencies/%d/edit", m.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "660px",
				DeleteURL:  fmt.Sprintf("/currencies/%d/delete", m.ID),
				DeleteName: m.IsoCode,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.currency.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/currencies/new", CreateLabel: t("masters.currency.new"),
			CreateTargetModal2: true, ModalWidth: "660px",
			MaxWidth: "max-w-3xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.currency.column.iso_code")}, {Label: t("masters.currency.field.symbol")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "payments", EmptyTitle: t("masters.currency.empty_title"),
			EmptyMessage: t("masters.currency.empty_message"),
		}, rows),
		nil,
	)
}

func MonedaFormPage(data MonedaFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/currencies"
	if data.IsEdit {
		action = fmt.Sprintf("/currencies/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.currency.field.iso_code"), "iso_code", data.IsoCode, "text", true, fieldError(data.Errors, "iso_code")),
			masterFormInputErr(b, t("masters.currency.field.symbol"), "symbol", data.Symbol, "text", true, fieldError(data.Errors, "symbol")),
		}
		if data.Fields != nil {
			inner = append(inner[:2:2], data.Fields(b))
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.currency.edit"), false: t("masters.currency.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, append(inner, masterFormButtons(b, "/currencies", t))...)...),
			),
		)
	}
}

// ── Centro de Costo ───────────────────────────────────────────────────────────

func CentrosCostoListPage(data CentrosCostoListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.cost_center.edit")
	rows := make([]mi.H, len(data.CostCenters))
	for i, c := range data.CostCenters {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", c.ID)),
			ListCellLink(c.AccountingCode, fmt.Sprintf("/cost-centers/%d", c.ID)),
			ListCell(c.Name),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/cost-centers/%d/edit", c.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "700px",
				DeleteURL:  fmt.Sprintf("/cost-centers/%d/delete", c.ID),
				DeleteName: c.AccountingCode,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.cost_center.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/cost-centers/new", CreateLabel: t("masters.cost_center.new"),
			CreateTargetModal2: true, ModalWidth: "700px",
			MaxWidth: "max-w-3xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.cost_center.field.accounting_code")}, {Label: t("masters.cost_center.field.name")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "account_balance", EmptyTitle: t("masters.cost_center.empty_title"),
			EmptyMessage: t("masters.cost_center.empty_message"),
		}, rows),
		nil,
	)
}

func CentroCostoFormPage(data CentroCostoFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/cost-centers"
	if data.IsEdit {
		action = fmt.Sprintf("/cost-centers/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.cost_center.field.accounting_code"), "accounting_code", data.AccountingCode, "text", true, fieldError(data.Errors, "accounting_code")),
			masterFormInputErr(b, t("masters.cost_center.field.name"), "name", data.Name, "text", true, fieldError(data.Errors, "name")),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		inner = append(inner, masterFormButtons(b, "/cost-centers", t))
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.cost_center.edit"), false: t("masters.cost_center.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, inner...)...),
			),
		)
	}
}

// ── Marca ─────────────────────────────────────────────────────────────────────

func MarcasListPage(data MarcasListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.brand.edit")
	rows := make([]mi.H, len(data.Brands))
	for i, m := range data.Brands {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", m.ID)),
			ListCellLink(m.Name, fmt.Sprintf("/brands/%d", m.ID)),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/brands/%d/edit", m.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "600px",
				DeleteURL:  fmt.Sprintf("/brands/%d/delete", m.ID),
				DeleteName: m.Name,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.brand.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/brands/new", CreateLabel: t("masters.brand.new"),
			CreateTargetModal2: true, ModalWidth: "600px",
			MaxWidth: "max-w-2xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.brand.field.name")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "label", EmptyTitle: t("masters.brand.empty_title"),
			EmptyMessage: t("masters.brand.empty_message"),
		}, rows),
		nil,
	)
}

func MarcaFormPage(data MarcaFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/brands"
	if data.IsEdit {
		action = fmt.Sprintf("/brands/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.brand.field.code"), "code", data.Code, "text", true, fieldError(data.Errors, "code")),
			masterFormInputErr(b, t("masters.brand.field.name"), "name", data.Name, "text", true, fieldError(data.Errors, "name")),
		}
		if data.Fields != nil {
			inner = append(inner[:2:2], data.Fields(b))
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		inner = append(inner, masterFormButtons(b, "/brands", t))
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.brand.edit"), false: t("masters.brand.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, inner...)...),
			),
		)
	}
}

// ── Custodio ──────────────────────────────────────────────────────────────────

func CustodiosListPage(data CustodiosListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.custodian.edit")
	activeLabel, inactiveLabel := t("status.active"), t("status.inactive")
	rows := make([]mi.H, len(data.Custodios))
	for i, c := range data.Custodios {
		active := c.Active // capture for closure
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", c.ID)),
			ListCellMuted(c.SubjectType+": "+c.TaxId),
			ListCellLink(c.LegalName, fmt.Sprintf("/custodians/%d", c.ID)),
			ListCellMuted(c.Email),
			ListCellRich(func(b *mi.Builder) mi.Node {
				if active {
					return Badge(activeLabel, "success")(b)
				}
				return Badge(inactiveLabel, "default")(b)
			}),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/custodians/%d/edit", c.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle,
				DeleteURL:      fmt.Sprintf("/custodians/%d/delete", c.ID),
				DeleteName:     c.LegalName,
				CSRFToken:      data.CSRFToken,
				T:              t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.custodian.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/custodians/new", CreateLabel: t("masters.custodian.new"),
			CreateTargetModal2: true},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.custodian.column.doc_type")}, {Label: t("masters.custodian.field.legal_name")}, {Label: t("masters.custodian.field.email")}, {Label: t("status.active")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "person_outline", EmptyTitle: t("masters.custodian.empty_title"),
			EmptyMessage: t("masters.custodian.empty_message"),
		}, rows),
		nil,
	)
}

func CustodioFormPage(data CustodioFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/custodians"
	if data.IsEdit {
		action = fmt.Sprintf("/custodians/%d", data.ID)
	}
	tipoOptions := []struct{ V, L string }{
		{"DNI", "DNI"}, {"CI", "CI"}, {"RUT", "RUT"}, {"RUC", "RUC"},
	}
	return func(b *mi.Builder) mi.Node {
		cbClass := "h-4 w-4 rounded border-gray-300 text-indigo-600"
		cbNode := func() mi.Node {
			if data.Active {
				return b.Input(mi.Type("checkbox"), mi.Name("active"), mi.ID("active"), mi.Class(cbClass), mi.Checked())
			}
			return b.Input(mi.Type("checkbox"), mi.Name("active"), mi.ID("active"), mi.Class(cbClass))
		}()
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormSelect(b, t("masters.custodian.field.document_type"), "subject_type", data.SubjectType, true, tipoOptions, t),
			masterFormInputErr(b, t("masters.custodian.field.legal_name"), "legal_name", data.LegalName, "text", true, fieldError(data.Errors, "legal_name")),
			masterFormInputErr(b, t("masters.custodian.field.tax_id"), "tax_id", data.TaxId, "text", true, fieldError(data.Errors, "tax_id")),
			masterFormInputErr(b, t("masters.custodian.field.alias"), "alias", data.Alias, "text", false, fieldError(data.Errors, "alias")),
			masterFormInputErr(b, t("masters.custodian.field.email"), "email", data.Email, "email", true, fieldError(data.Errors, "email")),
			masterFormInputErr(b, t("masters.custodian.field.phone"), "phone", data.Phone, "tel", false, fieldError(data.Errors, "phone")),
			masterFormInputErr(b, t("masters.custodian.field.address"), "address", data.Address, "text", false, fieldError(data.Errors, "address")),
			b.Div(mi.Class("mb-4 flex items-center gap-2"), cbNode,
				b.Label(mi.For("active"), mi.Class("text-sm font-medium text-gray-700 dark:text-gray-300"), t("status.active")),
			),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		inner = append(inner, masterFormButtons(b, "/custodians", t))
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.custodian.edit"), false: t("masters.custodian.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, inner...)...),
			),
		)
	}
}

// ── Alerta ────────────────────────────────────────────────────────────────────

func AlertasMasterListPage(data AlertasListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.alert_type.edit")
	rows := make([]mi.H, len(data.Alertas))
	for i, a := range data.Alertas {
		typeLabel := AlertaTypeLabel(a.Type, t)
		critLabel := AlertaCriticalityLabel(a.Criticality, t)
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", a.ID)),
			ListCellLink(typeLabel, fmt.Sprintf("/alert-types/%d", a.ID)),
			ListCell(a.Description),
			ListCellRich(func(b *mi.Builder) mi.Node {
				v := map[string]string{"High": "danger", "Medium": "warning", "Low": "success"}[a.Criticality]
				if v == "" {
					v = "default"
				}
				return Badge(critLabel, v)(b)
			}),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/alert-types/%d/edit", a.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "700px",
				DeleteURL:  fmt.Sprintf("/alert-types/%d/delete", a.ID),
				DeleteName: typeLabel,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.alert_type.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/alert-types/new", CreateLabel: t("masters.alert_type.new"),
			CreateTargetModal2: true, ModalWidth: "700px"},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.alert_type.field.type")}, {Label: t("masters.alert_type.field.description")}, {Label: t("assets.forms.asset.field.criticality")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "notifications_none", EmptyTitle: t("masters.alert_type.empty_title"),
			EmptyMessage: t("masters.alert_type.empty_message"),
		}, rows),
		nil,
	)
}

func AlertaMasterFormPage(data AlertaMasterFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/alert-types"
	if data.IsEdit {
		action = fmt.Sprintf("/alert-types/%d", data.ID)
	}
	typeOptions := []struct{ V, L string }{
		{"Maintenance", t("alert_type.category.maintenance")},
		{"Security", t("alert_type.category.security")},
		{"Operational", t("alert_type.category.operational")},
	}
	critOptions := []struct{ V, L string }{
		{"High", t("alert_type.criticality.high")},
		{"Medium", t("alert_type.criticality.medium")},
		{"Low", t("alert_type.criticality.low")},
	}
	return func(b *mi.Builder) mi.Node {
		descField := b.Div(mi.Class("mb-4"),
			b.Label(mi.For("description"),
				mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("masters.alert_type.field.description")+" *"),
			b.Textarea(
				mi.Name("description"), mi.ID("description"), mi.Required(),
				mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-indigo-500"),
				mi.Attr("rows", "3"),
				data.Description,
			),
		)
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormSelect(b, t("masters.alert_type.field.type"), "type", data.Type, true, typeOptions, t),
			descField,
			masterFormSelect(b, t("assets.forms.asset.field.criticality"), "criticality", data.Criticality, true, critOptions, t),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.alert_type.edit"), false: t("masters.alert_type.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, append(inner, masterFormButtons(b, "/alert-types", t))...)...),
			),
		)
	}
}

// ── PropDefinition ────────────────────────────────────────────────────────────

// propTipoOptionsList builds PropDef's data_type dropdown options fresh
// per request (not a package var -- var initializers run before any
// request-scoped t is available). Values are the English canonical
// tokens PropDefCreateSchema validates and propDefEnumValueToXolu
// bridges to xolu's Spanish storage; labels resolve via the existing
// prop_definition.data_type.* keys.
func propTipoOptionsList(t func(key string, args ...any) string) []struct{ V, L string } {
	return []struct{ V, L string }{
		{"numeric", t("prop_definition.data_type.numeric")},
		{"alphanumeric", t("prop_definition.data_type.alphanumeric")},
		{"date", t("prop_definition.data_type.date")},
		{"datetime", t("prop_definition.data_type.datetime")},
		{"time", t("prop_definition.data_type.time")},
		{"boolean", t("prop_definition.data_type.boolean")},
		{"single_select", t("prop_definition.data_type.single_select")},
		{"multi_select", t("prop_definition.data_type.multi_select")},
		{"system_entity", t("prop_definition.data_type.system_entity")},
		{"external_api", t("prop_definition.data_type.external_api")},
	}
}

func PropDefsListPage(data PropDefsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.propdef.edit")
	rows := make([]mi.H, len(data.Props))
	for i, p := range data.Props {
		label := PropDefDataTypeLabel(p.DataType, t)
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", p.ID)),
			ListCellLink(p.ShortName, fmt.Sprintf("/properties/%d", p.ID)),
			ListCellRich(func(b *mi.Builder) mi.Node {
				return Badge(label, "info")(b)
			}),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/properties/%d/edit", p.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "700px",
				DeleteURL:  fmt.Sprintf("/properties/%d/delete", p.ID),
				DeleteName: p.ShortName,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.propdef.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/properties/new", CreateLabel: t("masters.propdef.new"),
			CreateTargetModal2: true, ModalWidth: "700px",
			MaxWidth: "max-w-3xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.propdef.column.short_name")}, {Label: t("masters.propdef.column.type")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "tune", EmptyTitle: t("masters.propdef.empty_title"),
			EmptyMessage: t("masters.propdef.empty_message"),
		}, rows),
		nil,
	)
}

func PropDefFormPage(data PropDefFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/properties"
	if data.IsEdit {
		action = fmt.Sprintf("/properties/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		opts := []interface{}{
			mi.Name("tipo_dato"), mi.ID("tipo_dato"), mi.Required(),
			mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"),
			b.Option(mi.Value(""), t("masters.propdef.select_type")),
		}
		for _, o := range propTipoOptionsList(t) {
			attrs := []interface{}{mi.Value(o.V)}
			if o.V == data.DataType {
				attrs = append(attrs, mi.Selected())
			}
			if o.V == "external_api" {
				attrs = append(attrs, mi.Attr("disabled", "true"))
			}
			opts = append(opts, b.Option(append(attrs, o.L)...))
		}
		tipoField := b.Div(mi.Class("mb-4"),
			b.Label(mi.For("tipo_dato"),
				mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("masters.propdef.field.data_type")+" *"),
			b.Select(opts...),
		)
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.propdef.field.short_name"), "nombre_corto", data.ShortName, "text", true, fieldError(data.Errors, "nombre_corto")),
			tipoField,
			masterFormInputErr(b, t("masters.propdef.field.default_value"), "valor_defecto", data.DefaultValue, "text", false, fieldError(data.Errors, "valor_defecto")),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.propdef.edit"), false: t("masters.propdef.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, append(inner, masterFormButtons(b, "/properties", t))...)...),
			),
		)
	}
}

// ── Modelo (P-5) ──────────────────────────────────────────────────────────────

func ModelosListPage(data ModelosListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.model.edit")
	rows := make([]mi.H, len(data.Modelos))
	for i, m := range data.Modelos {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", m.ID)),
			ListCellLink(m.Name, fmt.Sprintf("/models/%d", m.ID)),
			ListCellMuted(m.BrandName),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/models/%d/edit", m.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "660px",
				DeleteURL:  fmt.Sprintf("/models/%d/delete", m.ID),
				DeleteName: m.Name,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.model.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/models/new", CreateLabel: t("masters.model.new"),
			CreateTargetModal2: true, ModalWidth: "660px",
			MaxWidth: "max-w-3xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.model.column.name")}, {Label: t("masters.model.column.brand")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "inventory_2", EmptyTitle: t("masters.model.empty_title"),
			EmptyMessage: t("masters.model.empty_message"),
		}, rows),
		nil,
	)
}

func ModeloFormPage(data ModeloFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/models"
	if data.IsEdit {
		action = fmt.Sprintf("/models/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		// Marca dropdown
		marcaOpts := []interface{}{
			mi.Name("marca_id"), mi.ID("marca_id"), mi.Required(),
			mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"),
			b.Option(mi.Value(""), t("masters.model.select_brand")),
		}
		for _, m := range data.Marcas {
			val := fmt.Sprintf("%d", m.ID)
			label := fmt.Sprintf("%s — %s", m.Code, m.Name)
			if m.ID == data.BrandID {
				marcaOpts = append(marcaOpts, b.Option(mi.Value(val), mi.Selected(), label))
			} else {
				marcaOpts = append(marcaOpts, b.Option(mi.Value(val), label))
			}
		}
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			b.Div(mi.Class("mb-4"),
				b.Label(mi.For("marca_id"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("masters.model.field.brand")),
				b.Select(marcaOpts...),
			),
			masterFormInputErr(b, t("masters.model.field.name"), "nombre", data.Name, "text", true, fieldError(data.Errors, "nombre")),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		inner = append(inner, masterFormButtons(b, "/models", t))
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.model.edit"), false: t("masters.model.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, inner...)...),
			),
		)
	}
}

// ── Ubicación (P-6) ───────────────────────────────────────────────────────────

func UbicacionesListPage(data UbicacionesListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	editTitle := t("masters.location.edit")
	rows := make([]mi.H, len(data.Locations))
	for i, u := range data.Locations {
		rows[i] = ListRow(i,
			ListCellMuted(fmt.Sprintf("%d", u.ID)),
			ListCellLink(u.ShortName, fmt.Sprintf("/locations/%d", u.ID)),
			ListCellMuted(u.RegionDesc),
			ListActions(ListActionsConfig{
				EditURL:  fmt.Sprintf("/locations/%d/edit", u.ID),
				EditHTMX: true, EditTargetModal2: true,
				EditModalTitle: editTitle, EditModalWidth: "660px",
				DeleteURL:  fmt.Sprintf("/locations/%d/delete", u.ID),
				DeleteName: u.ShortName,
				CSRFToken:  data.CSRFToken,
				T:          t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.location.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/locations/new", CreateLabel: t("masters.location.new"),
			CreateTargetModal2: true, ModalWidth: "660px",
			MaxWidth: "max-w-3xl",
		},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: "ID"}, {Label: t("masters.location.column.name")}, {Label: t("masters.location.field.region")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "place", EmptyTitle: t("masters.location.empty_title"),
			EmptyMessage: t("masters.location.empty_message"),
		}, rows),
		nil,
	)
}

func UbicacionFormPage(data UbicacionFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	action := "/locations"
	if data.IsEdit {
		action = fmt.Sprintf("/locations/%d", data.ID)
	}
	return func(b *mi.Builder) mi.Node {
		regionOpts := []interface{}{
			mi.Name("region_id"), mi.ID("region_id"),
			mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"),
			b.Option(mi.Value(""), t("masters.location.select_no_region")),
		}
		for _, r := range data.Regiones {
			val := fmt.Sprintf("%d", r.ID)
			if r.ID == data.RegionID {
				regionOpts = append(regionOpts, b.Option(mi.Value(val), mi.Selected(), r.Description))
			} else {
				regionOpts = append(regionOpts, b.Option(mi.Value(val), r.Description))
			}
		}
		inner := []interface{}{
			b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
			masterFormError(b, data.Error),
			masterFormInputErr(b, t("masters.location.field.short_name"), "short_name", data.ShortName, "text", true, fieldError(data.Errors, "short_name")),
			masterFormInputErr(b, t("masters.location.field.description"), "description", data.Description, "text", false, fieldError(data.Errors, "description")),
			b.Div(mi.Class("mb-4"),
				b.Label(mi.For("region_id"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("masters.location.field.region")),
				b.Select(regionOpts...),
			),
		}
		if data.IsPartial {
			return masterPartialFormWrapper(b, "entity-edit-form", action, inner, t)
		}
		inner = append(inner, masterFormButtons(b, "/locations", t))
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"),
				map[bool]string{true: t("masters.location.edit"), false: t("masters.location.new")}[data.IsEdit]),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(append([]interface{}{mi.Method("post"), mi.Attr("action", action), mi.Class("space-y-1")}, inner...)...),
			),
		)
	}
}

// ── DocumentLinksField (P-16) ─────────────────────────────────────────────────

// DocumentLink is a single entry in a document links list.
type DocumentLink struct {
	Name string
	URL  string
	Type string
}

// DocumentLinksField renders an add/remove list of document link rows.
// prefix must be unique per page (e.g. "loan_links", "lifecycle_links").
// existing holds the current saved links to pre-populate the form.
func DocumentLinksField(b *mi.Builder, label, prefix string, existing []DocumentLink, t func(key string, args ...any) string) mi.Node {
	const rowClass = "flex gap-2 items-start mb-2 doc-link-row"
	const inputClass = "px-2 py-1 border border-gray-300 dark:border-gray-600 rounded text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
	namePlaceholder, urlPlaceholder, typePlaceholder, addLinkLabel := "Name", "URL", "Type (optional)", "+ Add link"
	if t != nil {
		namePlaceholder, urlPlaceholder, typePlaceholder, addLinkLabel = t("field.name"), t("field.url"), t("field.type_optional"), t("action.add_link")
	}

	buildRow := func(nombre, url, tipo string) mi.Node {
		return b.Div(mi.Class(rowClass),
			b.Input(mi.Type("text"), mi.Name(prefix+"_nombre[]"),
				mi.Attr("placeholder", namePlaceholder), mi.Attr("value", nombre),
				mi.Class(inputClass+" flex-1")),
			b.Input(mi.Type("url"), mi.Name(prefix+"_url[]"),
				mi.Attr("placeholder", urlPlaceholder), mi.Attr("value", url),
				mi.Class(inputClass+" flex-2")),
			b.Input(mi.Type("text"), mi.Name(prefix+"_tipo[]"),
				mi.Attr("placeholder", typePlaceholder), mi.Attr("value", tipo),
				mi.Class(inputClass+" w-28")),
			b.Button(mi.Type("button"),
				mi.Class("text-red-500 hover:text-red-700 text-sm px-2"),
				mi.Attr("onclick", docLinkRowRemoveOnclick),
				"✕"),
		)
	}

	rows := []interface{}{}
	for _, l := range existing {
		rows = append(rows, buildRow(l.Name, l.URL, l.Type))
	}
	// Always one empty row if none exist
	if len(existing) == 0 {
		rows = append(rows, buildRow("", "", ""))
	}

	containerID := prefix + "_container"

	return b.Div(mi.Class("mb-4"),
		b.Label(mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"), label),
		b.Div(append([]interface{}{mi.ID(containerID)}, rows...)...),
		b.Button(mi.Type("button"),
			mi.Class("mt-1 text-sm text-indigo-600 dark:text-indigo-400 hover:underline"),
			mi.Attr("onclick", fmt.Sprintf(`
(function(){
  var c=document.getElementById('%s');
  var src=c._rowTemplate||c.querySelector('.doc-link-row');
  if(!src){return;}
  var t=src.cloneNode(true);
  t.querySelectorAll('input').forEach(function(i){i.value='';});
  c.appendChild(t);
})()`, containerID)),
			addLinkLabel,
		),
	)
}

// ── PropAssociation (P-18) ────────────────────────────────────────────────────

func PropAssocsListPage(data PropAssocsListData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	rows := make([]mi.H, len(data.Assocs))
	for i, a := range data.Assocs {
		rows[i] = ListRow(i,
			ListCellLink(a.ShortName, fmt.Sprintf("/properties/%d", a.PropDefID)),
			ListCellMuted(PropAssocTargetEntityLabel(a.TargetEntity, t)),
			ListCellMuted(fmt.Sprintf("%d", a.EntityID)),
			ListCellMuted(PropAssocRequirementLabel(a.Requirement, t)),
			ListActions(ListActionsConfig{
				DeleteURL: fmt.Sprintf("/properties/associations/%d/delete", a.ID),
				CSRFToken: data.CSRFToken,
				T:         t,
			}),
		)
	}
	return ListPage(
		ListPageConfig{Title: t("masters.propassoc.list_title"), Subtitle: t("masters.record_count", data.Total),
			CreateURL: "/properties/associations/new", CreateLabel: t("masters.propassoc.new"),
			CreateTargetModal2: true, ModalWidth: "700px"},
		nil,
		ListTable(ListTableConfig{
			Columns:   []ListColumn{{Label: t("masters.propassoc.column.property")}, {Label: t("masters.propassoc.column.target")}, {Label: t("masters.propassoc.field.entity_id")}, {Label: t("masters.propassoc.field.requirement")}, {Label: t("asset.list.actions"), Right: true}},
			EmptyIcon: "link", EmptyTitle: t("masters.propassoc.empty_title"),
			EmptyMessage: t("masters.propassoc.empty_message"),
		}, rows),
		nil,
	)
}

func PropAssocFormPage(data PropAssocFormData, t func(key string, args ...any) string) mi.H {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	// "por_regla"/rule-based requirement was dropped: PropAssocCreateSchema
	// only ever validated Enum("required", "optional") -- selecting it
	// always failed at the backend, matching the original "(próximamente)"
	// / "coming soon" label's own admission that it wasn't functional yet.
	oblOptions := []struct{ V, L string }{
		{"required", t("masters.propassoc.requirement.required")},
		{"optional", t("masters.propassoc.requirement.optional")},
	}
	destOptions := []struct{ V, L string }{
		{"asset_type", t("masters.propassoc.target_entity.asset_type")},
		{"asset", t("masters.propassoc.target_entity.asset")},
	}
	return func(b *mi.Builder) mi.Node {
		propOpts := []interface{}{
			mi.Name("prop_definition_id"), mi.ID("prop_definition_id"), mi.Required(),
			mi.Class("mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-sm"),
			b.Option(mi.Value(""), t("masters.propassoc.select_property")),
		}
		for _, p := range data.PropDefs {
			val := fmt.Sprintf("%d", p.ID)
			label := fmt.Sprintf("%s (%s)", p.ShortName, PropDefDataTypeLabel(p.DataType, t))
			if p.ID == data.PropDefID {
				propOpts = append(propOpts, b.Option(mi.Value(val), mi.Selected(), label))
			} else {
				propOpts = append(propOpts, b.Option(mi.Value(val), label))
			}
		}
		return b.Div(mi.Class("space-y-6"),
			b.H1(mi.Class("text-2xl font-bold text-gray-900 dark:text-white"), t("masters.propassoc.form_title")),
			b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 max-w-lg"),
				b.Form(mi.Method("post"), mi.Attr("action", "/properties/associations"), mi.Class("space-y-1"),
					b.Input(mi.Type("hidden"), mi.Name("_csrf"), mi.Attr("value", data.CSRFToken)),
					masterFormError(b, data.Error),
					b.Div(mi.Class("mb-4"),
						b.Label(mi.For("prop_definition_id"), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), t("masters.propassoc.field.property")),
						b.Select(propOpts...),
					),
					masterFormSelect(b, t("masters.propassoc.field.target_entity"), "entidad_destino", data.TargetEntity, true, destOptions, t),
					masterFormInputErr(b, t("masters.propassoc.field.entity_id"), "entidad_id", fmt.Sprintf("%d", data.EntityID), "number", true, fieldError(data.Errors, "entidad_id")),
					masterFormSelect(b, t("masters.propassoc.field.requirement"), "obligatoriedad", data.Requirement, true, oblOptions, t),
					masterFormButtons(b, "/properties/associations", t),
				),
			),
		)
	}
}

// ── Property value rendering (P-19) ──────────────────────────────────────────

// PropValuesSection renders a form section for custom property values.
// entries are the resolved associations with current values.
// PropValuesSection and propValueField are currently dead code
// (confirmed by full-tree grep, T-18 Stage 9 tier-1) -- no live route
// calls them. Fixed to the same standard as every live listbox anyway
// (English enum values, t-resolved labels), so nothing rediscovers
// this gap if/when the feature gets wired up.
func PropValuesSection(b *mi.Builder, entries []PropValueEntry, t func(key string, args ...any) string) mi.Node {
	if t == nil {
		t = func(key string, args ...any) string { return key }
	}
	if len(entries) == 0 {
		return b.Span() // nothing to render
	}

	fields := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		fields = append(fields, propValueField(b, e, t))
	}

	return b.Div(mi.Class("bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6"),
		b.H2(mi.Class("text-lg font-semibold text-gray-900 dark:text-white mb-4"), t("masters.propvalues.section_title")),
		b.Div(append([]interface{}{mi.Class("space-y-4")}, fields...)...),
	)
}

func propValueField(b *mi.Builder, e PropValueEntry, t func(key string, args ...any) string) mi.Node {
	name := "prop_values[" + e.ShortName + "]"
	label := e.ShortName
	if e.Requirement == "required" {
		label += " *"
	}
	const cls = "mt-1 w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"

	var input mi.Node
	switch e.DataType {
	case "numeric":
		input = b.Input(mi.Type("number"), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", e.CurrentValue))
	case "date":
		input = b.Input(mi.Type("date"), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", e.CurrentValue))
	case "datetime":
		input = b.Input(mi.Type("datetime-local"), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", e.CurrentValue))
	case "time":
		input = b.Input(mi.Type("time"), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", e.CurrentValue))
	case "boolean":
		checked := e.CurrentValue == "true" || e.CurrentValue == "1"
		const cbCls = "h-4 w-4 rounded border-gray-300 text-indigo-600"
		if checked {
			input = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class(cbCls), mi.Checked())
		} else {
			input = b.Input(mi.Type("checkbox"), mi.Name(name), mi.ID(name), mi.Class(cbCls))
		}
	default: // alphanumeric, single_select, multi_select, system_entity
		input = b.Input(mi.Type("text"), mi.Name(name), mi.ID(name), mi.Class(cls), mi.Attr("value", e.CurrentValue))
	}

	return b.Div(mi.Class("mb-2"),
		b.Label(mi.For(name), mi.Class("block text-sm font-medium text-gray-700 dark:text-gray-300"), label),
		input,
	)
}
