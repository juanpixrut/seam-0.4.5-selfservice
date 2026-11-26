package ui

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	mi "github.com/ha1tch/minty"
	fe "github.com/ha1tch/seam-ui/internal/formengine"
	workorderschemas "github.com/ha1tch/seam-ui/internal/formengine/schemas/workorders"
)

var errWorkOrderResponsibleUnavailable = errors.New(
	"work order responsible unavailable",
)

type workOrderAssignment struct {
	AssignmentType  string
	ResponsibleType string
	ResponsibleID   int64
	ResponsibleName string
}

// WorkOrderDirectAssignment devuelve el formulario del modal.
// No crea ni modifica órdenes.
func (h *UIHandler) WorkOrderDirectAssignment(w http.ResponseWriter, r *http.Request) {
	t := h.translateFunc(r)

	responsibleType := r.URL.Query().Get("responsible_type")

	fieldNames := map[string]string{
		"user":        "direct_user_id",
		"crew":        "direct_crew_id",
		"third_party": "direct_third_party_id",
	}

	fieldName, valid := fieldNames[responsibleType]
	if !valid {
		responsibleType = "user"
		fieldName = fieldNames[responsibleType]
	}

	values := map[string]any{
		"direct_responsible_type": responsibleType,
	}

	// Se usa para recuperar una selección confirmada anteriormente.
	if id, err := strconv.ParseInt(r.URL.Query().Get("responsible_id"), 10, 64); err == nil && id > 0 {
		values[fieldName] = id
	}

	engine := &fe.FormEngine{
		Translator: h.formEngineTranslator(),
	}

	rc := fe.RenderContext{
		Locale:    h.currentLocale(),
		Module:    "workorders",
		Form:      "direct-assignment",
		IsPartial: true,
	}

	schema := resolveSchema(r.Context(), "workorders", "direct-assignment", workorderschemas.Schema("direct-assignment"))

	content := engine.Render(r.Context(), schema, values, nil, rc)

	h.page.RenderPartial(w, WorkOrderDirectAssignmentPage(content, t))
}

func WorkOrderDirectAssignmentPage(content mi.H, t func(string, ...any) string) mi.H {
	const formID = "wo-direct-assignment-form"

	return func(b *mi.Builder) mi.Node {
		return b.Div(
			b.Style(mi.Raw(workOrderPickerModalStyle)),
			b.Form(
				mi.ID(formID),
				mi.Class("space-y-4"),
				mi.Attr(
					"onsubmit",
					"return SeamWorkOrderAssignment.confirm(this)",
				),

				content(b),

				b.P(
					mi.Attr("data-assignment-error", ""),
					mi.Attr("hidden", "hidden"),
					mi.Attr("role", "alert"),
					mi.Class("text-sm text-red-600 dark:text-red-400"),
					t("workorders.error.responsible_required"),
				),
			),

			b.Div(
				mi.Attr("data-modal-footer", ""),

				b.Button(
					t("action.cancel"),
					mi.Type("button"),
					mi.Class(
						"px-4 py-2 rounded-lg text-sm "+
							"bg-gray-100 dark:bg-gray-700 "+
							"text-gray-700 dark:text-gray-200",
					),
					mi.Attr("onclick", "SeamModal2.close()"),
				),

				b.Button(
					t("workorders.assignment.confirm"),
					mi.Type("submit"),
					mi.Attr("form", formID),
					mi.Class(
						"px-4 py-2 rounded-lg text-sm "+
							"bg-indigo-600 text-white hover:bg-indigo-700",
					),
				),
			),
		)
	}
}

// Resuelve el nombre utilizando la misma configuración que el picker.
// Para usuarios también conserva sus restricciones de visibilidad.
func (h *UIHandler) resolveWorkOrderResponsible(ctx context.Context, responsibleType string, responsibleID int64) (string, error) {
	sources := map[string]string{
		"user":        "entity:users",
		"crew":        "entity:crews",
		"third_party": "entity:outsourced",
	}

	source, valid := sources[responsibleType]
	if !valid || responsibleID <= 0 {
		return "", errWorkOrderResponsibleUnavailable
	}

	config, valid := pickerSearchConfigs[source]
	if !valid {
		return "", errWorkOrderResponsibleUnavailable
	}

	results, err := h.runPickerIDLookup(
		ctx,
		config,
		strconv.FormatInt(responsibleID, 10),
	)
	if err != nil {
		return "", err
	}

	if len(results) != 1 || results[0].ID != responsibleID {
		return "", errWorkOrderResponsibleUnavailable
	}

	name := strings.TrimSpace(results[0].Label)
	if name == "" {
		return "", errWorkOrderResponsibleUnavailable
	}

	return name, nil
}

// Conserva los valores enviados para reconstruir el formulario con errores.
// El nombre del responsable siempre se obtiene desde el servidor.
func (h *UIHandler) parseWorkOrderAssignment(r *http.Request, values map[string]any, errs map[string]string, t func(string, ...any) string) (workOrderAssignment, error) {
	result := workOrderAssignment{
		AssignmentType: strings.TrimSpace(
			r.PostForm.Get("assignment_type"),
		),
		ResponsibleType: strings.TrimSpace(
			r.PostForm.Get("responsible_type"),
		),
	}

	values["assignment_type"] = result.AssignmentType
	values["responsible_type"] = result.ResponsibleType
	values["responsible_id"] = ""

	valid := true

	switch result.AssignmentType {
	case "direct_assignment", workOrderBoardPosting:
	default:
		errs["assignment_type"] = t("core.errors.invalid_pattern")
		valid = false
	}

	switch result.ResponsibleType {
	case "user", "crew", "third_party":
	default:
		errs["responsible_type"] = t("core.errors.invalid_pattern")
		valid = false
	}

	if !valid || result.AssignmentType == workOrderBoardPosting {
		return result, nil
	}

	rawID := strings.TrimSpace(r.PostForm.Get("responsible_id"))
	values["responsible_id"] = rawID

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		errs["responsible_id"] = t(
			"workorders.error.responsible_required",
		)
		return result, nil
	}

	name, err := h.resolveWorkOrderResponsible(
		r.Context(),
		result.ResponsibleType,
		id,
	)

	if errors.Is(err, errWorkOrderResponsibleUnavailable) {
		errs["responsible_id"] = t("workorders.error.responsible_invalid")
		return result, nil
	}

	if err != nil {
		return result, err
	}

	result.ResponsibleID = id
	result.ResponsibleName = name

	return result, nil
}

const workOrderAssignmentScript = `
(function () {
  if (!window.SeamWorkOrderAssignment) {
    window.SeamWorkOrderAssignment = (function () {
      function mainForm() {
        return document.getElementById('wo-create-form');
      }

      function isDirect(form) {
        return form.elements.namedItem('assignment_type').value ===
          'direct_assignment';
      }

      function sync() {
        var form = mainForm();
        if (!form) return;

        var direct = isDirect(form);
        var row = form.querySelector(
          '[data-field-row="responsible_type"]'
        );

        // Conserva el select en el formulario para que se envíe.
        if (row) row.style.display = direct ? 'none' : '';

        var section = form.querySelector('[data-direct-assignment]');
        if (section) section.hidden = !direct;

        if (!direct) {
          form.elements.namedItem('responsible_id').value = '';
          var summary = form.querySelector('[data-assignment-summary]');
          summary.textContent = form.dataset.assignmentPending;
        }
      }

      function open() {
        var form = mainForm();
        if (!form || !isDirect(form)) return;

        var params = new URLSearchParams({
          responsible_type:
            form.elements.namedItem('responsible_type').value,
          responsible_id:
            form.elements.namedItem('responsible_id').value
        });

        SeamModal2.open(
          form.dataset.assignmentTitle,
          '/workorders/direct-assignment?' + params.toString(),
          { maxWidth: '42rem' }
        );
      }

      function confirm(modalForm) {
        var form = mainForm();
        if (!form || !isDirect(form)) return false;

        var type = modalForm.elements.namedItem(
          'direct_responsible_type'
        ).value;

        var names = {
          user: 'direct_user_id',
          crew: 'direct_crew_id',
          third_party: 'direct_third_party_id'
        };

        var input = modalForm.elements.namedItem(names[type]);
        var id = input ? input.value : '';

        if (!/^[1-9][0-9]*$/.test(id)) {
          modalForm.querySelector(
            '[data-assignment-error]'
          ).hidden = false;
          return false;
        }

        var picker = input.closest('[data-seam-picker]');
        var text = picker
          ? picker.querySelector('input[type="text"]')
          : null;

        form.elements.namedItem('responsible_type').value = type;
        form.elements.namedItem('responsible_id').value = id;

        var typeSelect = modalForm.elements.namedItem(
          'direct_responsible_type'
        );

        var typeLabel = typeSelect.options[
          typeSelect.selectedIndex
        ].text;

        form.querySelector(
          '[data-assignment-summary]'
        ).textContent = typeLabel + ': ' +
          (text && text.value ? text.value : '#' + id);

        SeamModal2.close();
        return false;
      }

      function init() {
		var form = mainForm();
		if (!form) return;

		sync();
	 }

      document.addEventListener('change', function (event) {
        var form = mainForm();

        if (form && event.target ===
            form.elements.namedItem('assignment_type')) {
          form.elements.namedItem('responsible_id').value = '';

          form.querySelector(
            '[data-assignment-summary]'
          ).textContent = form.dataset.assignmentPending;

          sync();
          if (isDirect(form)) open();
        }

        if (event.target.name === 'direct_responsible_type' &&
            event.target.closest('#wo-direct-assignment-form')) {
          var modalForm = event.target.form;

          // Descarta la selección temporal al cambiar de tipo.
          modalForm.querySelectorAll('[data-seam-picker]').forEach(
            function (picker) {
              picker.querySelectorAll('input').forEach(function (input) {
                input.value = '';
                input.setCustomValidity('');
              });
            }
          );

          modalForm.querySelector(
            '[data-assignment-error]'
          ).hidden = true;
        }
      });

      document.addEventListener('submit', function (event) {
        var form = mainForm();
        if (!form || event.target !== form) return;

        if (isDirect(form) &&
            !form.elements.namedItem('responsible_id').value) {
          event.preventDefault();
          open();
        }
      });

      document.addEventListener('htmx:load', init);

      return {
        init: init,
        open: open,
        confirm: confirm
      };
    }());
  }

  if (document.readyState === 'loading') {
    document.addEventListener(
      'DOMContentLoaded',
      window.SeamWorkOrderAssignment.init,
      { once: true }
    );
  } else {
    window.SeamWorkOrderAssignment.init();
  }
}());
`
const workOrderPickerModalStyle = `	#wo-template-picker-form .autoComplete_wrapper,
	#wo-direct-assignment-form .autoComplete_wrapper {
	width: 100%;
	max-width: none;
	}

	#wo-template-picker-form .autoComplete_wrapper > ul,
	#wo-direct-assignment-form .autoComplete_wrapper > ul {
	position: static;
	inset: auto;
	transform: none;
	width: 100%;
	max-height: min(16rem, 35vh);
	overflow-y: auto;
	box-sizing: border-box;
	margin-top: 0.5rem;
	}
`
