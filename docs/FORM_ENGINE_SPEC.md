# Seam AMS — Motor de Formularios Dinámico
## Especificación de Diseño

**Versión:** 0.4  
**Fecha:** Agosto 2026 (v0.4: T-10 widgets/extensions documented — signature, decimal, multi-select, barcode, gps, image, x-seam-unit, x-seam-input-mask, x-seam-custom-validator, x-seam-erp-code, x-seam-barcode-format; asset-picker and §9.2's format table corrected from stale content)  
**Estado:** Borrador interno  
**Repositorio:** github.com/ha1tch/seam (núcleo compartido entre módulos)

---

## 1. Propósito y Contexto

El motor de formularios dinámico es un componente del núcleo compartido de Seam AMS que
reemplaza los formularios estáticos definidos en tiempo de compilación por un sistema donde
la estructura, el orden visual, las validaciones y los widgets de cada formulario se derivan
en tiempo de ejecución a partir de un JSON Schema almacenado en xolu.

### 1.1 Motivación

Los formularios actuales de Seam AMS (Assets, Sensores, Maestros) tienen su estructura
codificada en Go. Añadir un campo nuevo, reorganizar tabs o cambiar una validación requiere
modificar código, recompilar y redesplegar. Esto impide la modalidad de onboarding DIY
definida en el modelo comercial, donde el operador configura el sistema autónomamente sin
intervención de consultoría.

El motor dinámico resuelve esto usando JSON Schema como única fuente de verdad para:

- Estructura visual (tabs, secciones, grupos, orden de campos)
- Tipos de widget a renderizar
- Reglas de validación (obligatoriedad, formatos, rangos, patrones)
- Fuentes de datos dinámicas (entidades del sistema)
- Comportamientos interactivos (popovers, listas repetibles)

### 1.2 Infraestructura de soporte

El motor se apoya en infraestructura existente y no requiere nuevas dependencias:

| Componente | Rol |
|------------|-----|
| **queryfy** (`github.com/ha1tch/queryfy`) | Compilación y validación de schemas en el backend. Importación/exportación JSON Schema con round-trip verificado. |
| **xolu** (`github.com/ha1tch/xolu`) | Almacenamiento de schemas por tipo de asset/sensor. Los schemas viajan con la definición de la entidad. |
| **minty** (`github.com/ha1tch/minty`) | Renderizado HTML del formulario en el backend a partir del schema compilado. |
| **htmx** (vendoreado en `web/static/vendor/`) | Interactividad del formulario sin JavaScript propio: selects dinámicos, listas repetibles. |
| **SeamModal / SeamModal2** | Contenedores modales existentes. El motor renderiza formularios en ambos contextos. |
| **SeamPopover** | Popover contextual existente para preview de entidades FK. |

### 1.3 Alcance del MVP

El MVP cubre la expresión completa de los formularios actuales de Seam AMS en JSON Schema,
de modo que el motor dinámico pueda renderizar el formulario de Assets (cuatro tabs, secciones
con grupos de campos, todos los tipos de widget existentes) sin pérdida de funcionalidad
respecto a la implementación estática actual.

No está en el alcance del MVP:

- Editor visual de schemas (drag-and-drop de campos)
- Validación condicional compleja entre campos (dependencias cross-field)
- Schemas con más de dos niveles de anidamiento en `repeatable`

---

## 2. JSON Schema con Extensiones Seam

El motor usa JSON Schema draft-07 estándar como base, extendido con propiedades
prefijadas `x-seam-*`. La propiedad `attachable` en secciones no es una extensión
`x-seam-*` — es una propiedad directa de la definición de sección en `x-seam-form`. Las propiedades estándar (`title`, `type`, `format`, `enum`,
`minimum`, `maximum`, `minLength`, `maxLength`, `pattern`, `required`) son reconocidas
directamente por queryfy y no requieren extensión.

### 2.1 Tabla de extensiones

| Extensión | Nivel | Propósito |
|-----------|-------|-----------|
| `x-seam-form` | Schema raíz | Define la estructura visual: tabs, secciones, orden de campos, configuración del formulario como un todo. |
| `x-seam-input` | Propiedad | Fuerza un tipo de widget específico cuando el tipo JSON estándar es ambiguo o insuficiente. |
| `x-seam-source` | Propiedad | Fuente de datos dinámica para widgets de selección (selects FK, asset-picker, multi-select). |
| `x-seam-help` | Propiedad | Texto de ayuda asociado al campo. Complementado por `x-seam-help-style`. |
| `x-seam-fk-popover` | Propiedad | Habilita el popover de preview para selects FK. Valor: URL base de la entidad. |
| `x-seam-relation` | Propiedad | Tipo de arista del grafo de assets para el widget `asset-picker`. Valores: `contains`, `uses`. |
| `x-seam-layout` | Propiedad | (0.4.1-pre6) En un `repeatable`: `row` (por defecto, todas las celdas en una linea) o `form` (cada fila es una tarjeta con las celdas en una grilla, cada una bajo su etiqueta). |
| `x-seam-browse` | Propiedad | (0.4.1-pre6) En un `any-picker` con `x-seam-selector`: `button` (por defecto, boton con icono y texto), `icon` (boton pequeno solo icono) o `link` (el enlace de texto original). |
| `x-seam-highlight` | Propiedad | (0.4.1-pre6) En `any-picker`, `icon-picker` y `asset-picker`: como se resalta la coincidencia: `color` (por defecto, negrita en el color primario del tema), `background` (marcador translucido) o `none`. |
| `x-seam-add-label` | Propiedad | (0.4.1-pre8) En un `repeatable`: texto del boton de agregar (texto literal, no una clave i18n). |
| `x-seam-add-icon` | Propiedad | (0.4.1-pre8) En un `repeatable`: nombre del icono (Material Icons) del boton de agregar. Solo letras minusculas, digitos y guion bajo; otro valor se ignora. |
| `x-seam-add-style` | Propiedad | (0.4.1-pre8) En un `repeatable`: `outline` (por defecto), `filled` (relleno), `link` (enlace de texto) o `icon` (solo icono; el texto pasa a ser el tooltip). |
| `x-seam-remove-icon` | Propiedad | (0.4.1-pre8) En un `repeatable`: icono del boton de quitar de cada fila. |
| `x-seam-remove-label` | Propiedad | (0.4.1-pre8) En un `repeatable`: texto junto al icono del boton de quitar (por defecto solo icono). |
| `x-seam-remove-color` | Propiedad | (0.4.1-pre8) En un `repeatable`: color del boton de quitar. Un nombre (`indigo`, `emerald`, `amber`, `rose`, `sky`, `violet`, `teal`, `slate`) o `#rgb`/`#rrggbb`; otro valor se ignora. |
| `x-seam-accent` | Propiedad | (0.4.1-pre8) En un `repeatable`: color de las filas, del boton de agregar y de la linea de anidacion. Mismos valores que `x-seam-remove-color`. Un repetible anidado puede tener el suyo. |
| `x-seam-reorder` | Propiedad | (0.4.1-pre8) En un `repeatable`: si es `true`, cada fila tiene un asa para arrastrar y se reordena con el puntero o con las flechas del teclado (Inicio y Fin van a los extremos). Apagado por defecto. Con nombres planos el navegador envia las filas en el nuevo orden; con nombres indexados cada fila envia ademas `nombre[i][_pos]` (se actualiza tras cada movimiento y al enviar) y `ParseIndexedRepeatable` ordena por el y lo quita del resultado. Los repetibles anidados se reordenan por separado. |
| `minItems`, `maxItems` | Propiedad (palabras clave estándar de JSON Schema) | (0.4.1-pre8) En un `repeatable`: mínimo y máximo de filas. Con mínimo se muestran al menos ese número de filas (las que faltan, en blanco) y el botón de quitar de cada fila se desactiva al llegar a él; con máximo el botón de agregar se desactiva al llegar a él, y las filas que ya excedan el máximo se muestran todas. Debe cumplirse 0 <= minItems <= maxItems <= 1000 y maxItems >= 1, o el esquema se rechaza al leerlo. El formulario solo mantiene honestos los botones (`SeamRepeatable` en `app.js`, para ambos tipos de nombre): la comprobación que cuenta es la del servidor, `ValidatePostedBounds`, que cuenta las filas de ambos tipos de nombre sin contar las que tienen vacío el primer campo, y `RepeatableBoundsMessage` si el manejador cuenta sus propias filas. Sin ninguno la salida no cambia. |
| `x-seam-option-icons` | Propiedad | (0.4.1-pre8) En un `select`: mapa de valor de opción a nombre de Material Icon, que se muestra junto al texto de la opción en la caja y en la lista. Una opción de un `SourceResolver` puede traer su propio `Option.Icon`. El nombre debe ser minúsculas, dígitos y guiones bajos: se rechaza al leer el esquema, y se descarta en una opción que viene de datos. Lo dibuja el combo de búsqueda (`SeamSelect`): con `searchable_select` apagado la lista nativa no puede mostrar iconos y conserva solo el texto. Sin iconos la salida no cambia. |
| `x-seam-icon-position` | Propiedad | (0.4.1-pre8) En un `select` con iconos: `left` (por defecto; el icono antes del texto) o `right` (después del texto, al borde). Cualquier otro valor se rechaza al leer el esquema. |
| `x-seam-placeholder` | Propiedad | (0.4.1-pre3) Placeholder de un campo de texto, numero, textarea, any-picker o icon-picker, y texto de la opcion vacia de un select. Dentro de una fila de repetible tambien nombra la celda. |
| `x-seam-selector` | Propiedad | (T-97) Clave de una fuente del Selector (por ejemplo `users`); solo para `any-picker`. Si está presente, el widget agrega un botón Browse que abre el diálogo Selector. |

The extensions below were added in T-10 (2026-08). Per this repository's own
documentation convention (all new internal documentation in English —
existing Spanish specs are not retroactively translated), these entries and
the widget documentation in §4.2 that references them are written in
English, while the rest of this document remains in the Spanish it was
originally written in.

| Extension | Level | Purpose |
|-----------|-------|---------|
| `x-seam-unit` | Property | Static unit-of-measure suffix shown beside a field's control (e.g. `"°C"`, `"kg"`). Display-only — no conversion logic. Applied uniformly at the `renderFieldRow` layer, so it works with any widget, not just number/decimal. |
| `x-seam-input-mask` | Property | Client-side formatting mask for text-like widgets. Syntax: `9` = digit, `A` = letter (uppercased), `a` = letter (any case), `*` = alphanumeric, any other character is a literal auto-inserted as the operator types (e.g. `"999-999-9999"`). |
| `x-seam-custom-validator` | Property | Names a validator registered in `FormEngine.CustomValidators` (`func(value interface{}) error`). Checked by a separate pass (`ValidateCustomFields`) a caller runs alongside queryfy's own compiled-schema validation — not injected into the compiled schema itself. A field naming an unregistered validator fails closed (a schema-authoring typo becomes a visible error, not a silent pass). |
| `x-seam-erp-code` | Property | Pass-through identifier for an external ERP system's own code for this field (e.g. `"SAP:KOSTL"`). Inert at the formengine layer — no widget, no validation — until a future integration consumes it. |
| `x-seam-barcode-format` | Property | Selects the linear-barcode symbology for the `barcode` widget: `"EAN13"` (default), `"EAN8"`, `"UPC"`, `"UPCE"`. |

---

## 3. Extensión `x-seam-form`

Define la estructura visual del formulario completo. Se coloca en el nivel raíz del schema.

### 3.1 Estructura

```json
"x-seam-form": {
  "width": "2xl",
  "tabs": [
    {
      "id":    "tab-principal",
      "label": "assets.forms.asset.tab.principal",
      "sections": [
        {
          "id":     "basico",
          "label":  "assets.forms.asset.section.basico",
          "layout": "2-col",
          "fields": ["name", "type_id", "status", "criticality", "connectivity_tier"]
        },
        {
          "id":     "caracterizacion",
          "label":  "assets.forms.asset.section.caracterizacion",
          "layout": "2-col",
          "fields": ["brand_id", "model_id", "serial_1", "serial_2", "lot"]
        }
      ],
      "details": [
        {
          "id":       "sensores_vinculados",
          "label":    "assets.forms.asset.detail.sensores_vinculados.title",
          "source":   "/assets/{id}/sensors",
          "columns":  ["external_id", "tipo", "status", "last_seen"],
          "empty":    "assets.forms.asset.detail.sensores_vinculados.empty",
          "position": "bottom"
        }
      ]
    },
    {
      "id":    "tab-loc",
      "label": "assets.forms.asset.tab.loc",
      "sections": [
        {
          "id":     "ubicacion",
          "label":  "assets.forms.asset.section.ubicacion",
          "layout": "2-col-left",
          "fields": ["location_id", "region_id", "latitude", "longitude", "parent_id"]
        },
        {
          "id":     "custodia",
          "label":  "assets.forms.asset.section.custodia",
          "layout": "2-col-right",
          "fields": ["custodian_id", "tenure_type", "loan_expiry", "loan_links"]
        }
      ],
      "details": [
        {
          "id":       "custodio_history",
          "label":    "assets.forms.asset.detail.custodio_history.title",
          "source":   "/assets/{id}/custody-history",
          "columns":  ["fecha", "custodio", "calidad", "usuario"],
          "empty":    "assets.forms.asset.detail.custodio_history.empty",
          "position": "bottom"
        }
      ],
      "embeds": [
        {
          "id":       "sensor_binding",
          "label":    "assets.forms.asset.embed.sensor_binding.title",
          "endpoint": "/assets/{id}/sensors",
          "method":   "post",
          "trigger":  "button",
          "trigger_label": "assets.forms.asset.embed.sensor_binding.actions.bind",
          "mode":     "modal2",
          "schema":   "sensor_binding"
        }
      ]
    }
  ]
}
```

### 3.2 Propiedades de `x-seam-form`

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `module` | string | Default de `{módulo}` para resolución i18n (I18N_SPEC.md §4.1) -- p.ej. `"assets"`. Sobreescribible por `RenderContext.Module`. Opcional; sin este ni el override, la resolución de claves cae al fallback de texto literal. |
| `name` | string | Default de `{nombre_formulario}` para resolución i18n -- p.ej. `"asset"`. Sobreescribible por `RenderContext.Form`. Pensado para schemas reutilizables (p.ej. una tarjeta de widget) instanciados varias veces bajo distintos namespaces -- el schema declara un default razonable, el caller lo reemplaza sólo cuando esa instancia particular lo necesita. |
| `tabs` | array | Lista ordenada de tabs. Si ausente, el formulario se renderiza como lista plana de campos. |
| `modal_width` | string | Ancho del contenedor modal. Pasa a `SeamModal2.open()` como `maxWidth`. |
| `max_height` | string | Altura máxima del contenedor. |

### 3.3 Propiedades de cada tab

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `id` | string | Identificador único. Se usa como `id` del `<div>` del panel y como `data-tab` del botón. |
| `label` | string | Clave i18n del label visible en el botón del tab. |
| `sections` | array | Lista ordenada de secciones de campos editables dentro del tab. |
| `details` | array | Lista de paneles de detalle de sólo lectura dentro del tab. Opcional. |
| `embeds` | array | Lista de sub-formularios embebidos dentro del tab. Opcional. |

### 3.4 Propiedades de cada sección

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `id` | string | Identificador único de la sección. |
| `label` | string | Clave i18n del encabezado. Se renderiza como `<H3>` en `text-xs font-semibold text-gray-400 uppercase tracking-wider`. |
| `layout` | string | `"1-col"`, `"2-col"`, `"2-col-left"`, `"2-col-right"`. Valor por defecto: `"1-col"`. |
| `fields` | array | Lista ordenada de claves de propiedades del schema. El orden determina el orden de renderizado. |
| `attachable` | boolean | `true` = la sección acepta adjuntos de documentos. Al renderizar en modo edición, el motor emite el tray de adjuntos al pie de la sección, controlado por el `TrayPermission` correspondiente en el `RenderContext`. Por defecto `false`. |

### 3.5 Paneles de detalle (`details`)

Un panel de detalle muestra información relacionada con la entidad dentro del formulario,
en modo de sólo lectura. No genera datos al guardar el formulario principal. Se carga
mediante una llamada htmx al `source` cuando el tab se activa por primera vez.

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `id` | string | Identificador único del panel. |
| `label` | string | Clave i18n del título del panel. |
| `source` | string | URL del endpoint que retorna los datos. Puede contener `{id}` como placeholder del ID de la entidad. |
| `columns` | array | Lista de claves de columnas a mostrar. El label de cada columna se resuelve como `{módulo}.forms.{form}.detail.{id}.col.{columna}`. |
| `empty` | string | Clave i18n del mensaje de estado vacío. |
| `position` | string | `"bottom"` (debajo de las secciones del tab), `"right"` (columna lateral), `"inline"` (entre secciones). Valor por defecto: `"bottom"`. |
| `collapsible` | boolean | `true` = el panel puede colapsar/expandir. Por defecto `false`. |
| `load` | string | `"eager"` (al activar el tab) o `"lazy"` (al hacer scroll hasta el panel). Por defecto `"eager"`. |

**Ejemplo — historial de custodia:**

```json
{
  "id":       "custodio_history",
  "label":    "assets.forms.asset.detail.custodio_history.title",
  "source":   "/assets/{id}/custody-history",
  "columns":  ["fecha", "custodio", "calidad", "usuario"],
  "empty":    "assets.forms.asset.detail.custodio_history.empty",
  "position": "bottom",
  "collapsible": true
}
```

**Ejemplo — estado FSM:**

```json
{
  "id":       "fsm_state",
  "label":    "assets.forms.asset.detail.fsm_state.title",
  "source":   "/assets/{id}/fsm-state",
  "columns":  [],
  "position": "right",
  "load":     "eager"
}
```

### 3.6 Sub-formularios embebidos (`embeds`)

Un sub-formulario embebido permite ejecutar una acción sobre una entidad relacionada sin
salir del formulario principal. Tiene su propio endpoint, su propio ciclo de submit, y
opcionalmente su propio schema de campos.

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `id` | string | Identificador único del embed. |
| `label` | string | Clave i18n del título. |
| `endpoint` | string | URL del endpoint de acción. Puede contener `{id}`. |
| `method` | string | Método HTTP: `"post"`, `"put"`, `"delete"`. |
| `trigger` | string | Cómo se activa: `"button"` (botón explícito), `"inline"` (siempre visible), `"auto"` (se activa al cumplirse una condición). |
| `trigger_label` | string | Clave i18n del label del botón disparador (cuando `trigger: "button"`). |
| `mode` | string | Cómo se renderiza: `"modal2"` (abre SeamModal2), `"inline"` (se expande dentro del tab), `"drawer"` (panel lateral). Valor por defecto: `"modal2"`. |
| `schema` | string | Nombre del schema de campos del sub-formulario, resuelto desde xolu. Si ausente, el embed no tiene campos propios (es sólo una acción con confirmación). |
| `confirm` | string | Clave i18n del mensaje de confirmación antes de ejecutar la acción. Si ausente, no hay confirmación. |
| `refresh_detail` | string | ID de un panel `detail` del mismo tab que debe recargarse tras el submit exitoso. |

**Ejemplo — vincular sensor:**

```json
{
  "id":            "sensor_binding",
  "label":         "assets.forms.asset.embed.sensor_binding.title",
  "endpoint":      "/assets/{id}/sensors",
  "method":        "post",
  "trigger":       "button",
  "trigger_label": "assets.forms.asset.embed.sensor_binding.actions.bind",
  "mode":          "modal2",
  "schema":        "sensor_binding",
  "refresh_detail": "sensores_vinculados"
}
```

**Ejemplo — desvincular sensor (acción sin campos, sólo confirmación):**

```json
{
  "id":       "sensor_unbind",
  "label":    "assets.forms.asset.embed.sensor_binding.title",
  "endpoint": "/assets/{id}/sensors/{sensor_id}",
  "method":   "delete",
  "trigger":  "button",
  "trigger_label": "assets.forms.asset.embed.sensor_binding.actions.unbind",
  "mode":     "modal2",
  "confirm":  "assets.forms.asset.embed.sensor_binding.confirm.unbind",
  "refresh_detail": "sensores_vinculados"
}
```

**Ejemplo — asignar QR:**

```json
{
  "id":            "qr_assign",
  "label":         "assets.forms.asset.embed.qr_assign.title",
  "endpoint":      "/assets/{id}/qr",
  "method":        "post",
  "trigger":       "button",
  "trigger_label": "assets.forms.asset.embed.qr_assign.actions.assign",
  "mode":          "inline",
  "schema":        "qr_assign"
}
```

### 3.7 Relación entre `detail`, `embed` y el ciclo de vida del formulario

| Elemento | Genera datos al guardar | Tiene submit propio | Se carga con htmx | Requiere entidad existente |
|----------|------------------------|--------------------|--------------------|---------------------------|
| `section` | ✓ | — | — | — |
| `detail` | ✗ | — | ✓ | ✓ (sólo en modo edición) |
| `embed` | ✗ (directo) | ✓ | ✓ | ✓ (sólo en modo edición) |
| `tray` | ✗ | ✗ | ✓ lazy | ✓ (sólo en modo edición) |

El tray de adjuntos se renderiza al pie de cada sección con `attachable: true`, en modo
edición únicamente. Requiere `EntityType` + `EntityID` + `SectionID` del `RenderContext`
para construir la URL del endpoint htmx. Su visibilidad e interactividad se controlan
exclusivamente por el `TrayPermission` de la sección (§3c.8) — no por `HiddenSections`
ni `ReadOnlySections`, que afectan sólo a los campos de la sección.

Los paneles `detail` y sub-formularios `embed` sólo se renderizan en modo edición (`IsEdit: true`).
En modo creación, los tabs que sólo contienen `details` o `embeds` (sin `sections` con `fields`)
se ocultan o deshabilitan hasta que la entidad principal exista.

### 3.8 Layout de filas dentro de un tab

Las secciones dentro de un tab se organizan en filas. Una fila puede contener una, dos o
tres secciones colocadas lado a lado. Esto introduce un nivel de layout intermedio entre
el tab y la sección.

La propiedad `rows` reemplaza a `sections` cuando se necesita control de layout horizontal.
Cada fila es un objeto con `sections` (lista de IDs de sección) y opcionalmente `widths`
(lista de fracciones de ancho). Las definiciones de las secciones referenciadas se declaran
en el array `section_defs` del tab.

**Ejemplo A — dos secciones en la misma fila, anchos iguales:**

```json
{
  "id":    "tab-principal",
  "label": "assets.forms.asset.tab.principal",
  "rows": [
    {
      "sections": ["basico", "identificacion"]
    },
    {
      "sections": ["caracterizacion"]
    }
  ],
}
```

**Ejemplo B — secciones con anchos explícitos:**

```json
{
  "id":    "tab-loc",
  "label": "assets.forms.asset.tab.loc",
  "rows": [
    {
      "sections": ["custodio", "vencimientos"],
      "widths":   ["2/3",     "1/3"]
    },
    {
      "sections": ["loan_links"]
    }
  ],
  "section_defs": [
    {
      "id":     "basico",
      "label":  "assets.forms.asset.section.basico",
      "layout": "2-col",
      "fields": ["name", "type_id", "status", "criticality"]
    },
    {
      "id":     "identificacion",
      "label":  "assets.forms.asset.section.identificacion",
      "layout": "1-col",
      "fields": ["giai", "grai", "barcode_rfid"]
    },
    {
      "id":     "custodio",
      "label":  "assets.forms.asset.section.custodia",
      "layout": "1-col",
      "fields": ["custodian_id", "tenure_type"]
    },
    {
      "id":     "vencimientos",
      "label":  "assets.forms.asset.section.vencimientos",
      "layout": "1-col",
      "fields": ["loan_expiry", "warranty_end"]
    },
    {
      "id":     "loan_links",
      "label":  "assets.forms.asset.section.loan_links",
      "layout": "1-col",
      "fields": ["loan_links"]
    }
  ]
}
```

#### Propiedades de cada fila

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `sections` | array | Lista de 1 a 3 IDs de sección a renderizar lado a lado. Los IDs deben estar definidos en `section_defs`. |
| `widths` | array | Lista de fracciones de ancho para cada sección. Si ausente, el ancho se reparte en partes iguales. |

#### Valores de `widths`

| Valor | Clases Tailwind equivalentes | Descripción |
|-------|------------------------------|-------------|
| `"full"` | `col-span-12` | Ancho completo de la fila |
| `"1/2"` | `col-span-6` | Mitad |
| `"1/3"` | `col-span-4` | Un tercio |
| `"2/3"` | `col-span-8` | Dos tercios |
| `"1/4"` | `col-span-3` | Un cuarto |
| `"3/4"` | `col-span-9` | Tres cuartos |

El grid subyacente es de 12 columnas. Si los anchos de una fila no suman 12, el motor
distribuye el espacio restante entre las secciones sin ancho explícito.

#### Compatibilidad con `sections` directo

Si el tab usa `sections` en lugar de `rows`, el motor lo trata como una fila por sección,
cada una ocupando el ancho completo. Esto preserva la compatibilidad con los schemas
existentes y con los formularios de maestros simples.

#### Regla de collapse automático de filas

Si todos los campos de todas las secciones de una fila quedan ocultos (por visibilidad
condicional o por permisos), la fila entera se omite del HTML. No se emite ni el
contenedor de la fila.

### 3.9 Ancho del formulario

La propiedad `width` en `x-seam-form` controla el ancho total del contenedor del
formulario. Afecta tanto a la vista de página completa como al contenedor modal.

| Valor | Tailwind / CSS | Uso típico |
|-------|---------------|------------|
| `"sm"` | `max-w-lg` (~512px) | Formularios con pocos campos, maestros simples |
| `"md"` | `max-w-2xl` (~672px) | Formularios de maestros con más campos. **Valor por defecto.** |
| `"lg"` | `max-w-4xl` (~896px) | Formularios intermedios con 2 columnas |
| `"xl"` | `max-w-6xl` (~1152px) | Formularios complejos con tabs y múltiples secciones |
| `"2xl"` | `max-w-7xl` (~1280px) | Formularios muy ricos (equivalente al Asset actual) |
| `"fullscreen"` | `w-full h-full` | Formularios que requieren todo el ancho de pantalla: editores FSM, schemas, mapas embebidos |

```json
"x-seam-form": {
  "width":      "2xl",
  "max_height": "900px",
  "tabs": [ ... ]
}
```

Cuando `width` es `"fullscreen"`, el motor ignora `max_height` y el formulario ocupa
el viewport completo. En modo modal, `"fullscreen"` activa el SeamModal en lugar de
SeamModal2.

La propiedad `modal_width` existente se mantiene por compatibilidad pero queda deprecada
en favor de `width`. Si ambas están presentes, `width` tiene precedencia.

### 3.10 Formulario sin tabs

Si `x-seam-form` está ausente o no contiene `tabs`, el motor renderiza los campos en el
orden en que aparecen en `properties`, sin encabezados de sección ni filas. Este es el
comportamiento por defecto y es compatible con todos los formularios de maestros actuales
(Región, Moneda, Marca, Custodio, etc.) sin necesidad de modificación.

---

## 3b. Visibilidad Condicional

### 3b.1 Propósito

La visibilidad condicional permite que campos, secciones, filas y tabs aparezcan o se
oculten en función del valor actual de otros campos del mismo formulario. Es evaluada en
el cliente (JavaScript reactivo) y verificada en el servidor al validar (queryfy `if/then`).

### 3b.2 Propiedad `x-seam-visible-when` en campos

Se coloca en la definición de una propiedad del schema. El campo se omite del HTML cuando
la condición no se cumple.

```json
"loan_expiry": {
  "type":   "string",
  "format": "date",
  "title":  "assets.forms.asset.field.loan_expiry",
  "x-seam-visible-when": {
    "field": "tenure_type",
    "op":    "eq",
    "value": "Comodato"
  }
}
```

### 3b.3 Propiedad `visible_when` en secciones, filas y tabs

Se coloca dentro de la definición del elemento en `x-seam-form`. La semántica es idéntica
a `x-seam-visible-when` en campos.

```json
{
  "id":           "comodato",
  "label":        "assets.forms.asset.section.comodato",
  "fields":       ["loan_expiry", "loan_links"],
  "visible_when": {
    "field": "tenure_type",
    "op":    "eq",
    "value": "Comodato"
  }
}
```

```json
{
  "sections":    ["detalles_financieros"],
  "widths":      ["full"],
  "visible_when": {
    "field": "tenure_type",
    "op":    "in",
    "value": ["Venta", "Comodato"]
  }
}
```

### 3b.4 Operadores soportados

| Operador | Semántica | Valor esperado |
|----------|-----------|---------------|
| `eq` | Igual a | Escalar: string, number, boolean |
| `neq` | Distinto de | Escalar |
| `in` | El campo está en la lista | Array de escalares |
| `not_in` | El campo no está en la lista | Array de escalares |
| `empty` | El campo está vacío, es `null`, `""`, o `0` | — (sin `value`) |
| `not_empty` | El campo tiene un valor no vacío | — (sin `value`) |
| `gt` | Mayor que | Número |
| `gte` | Mayor o igual que | Número |
| `lt` | Menor que | Número |
| `lte` | Menor o igual que | Número |

### 3b.5 Condiciones compuestas

Para condiciones que combinan múltiples reglas se usan los contenedores `all_of` y `any_of`:

```json
"visible_when": {
  "all_of": [
    { "field": "tenure_type", "op": "eq",        "value": "Comodato" },
    { "field": "custodian_id",      "op": "not_empty"                      }
  ]
}
```

```json
"visible_when": {
  "any_of": [
    { "field": "status", "op": "eq", "value": "alert" },
    { "field": "status", "op": "eq", "value": "lost"  }
  ]
}
```

Los contenedores son anidables:

```json
"visible_when": {
  "all_of": [
    { "field": "status", "op": "neq", "value": "archivado" },
    {
      "any_of": [
        { "field": "tenure_type", "op": "eq", "value": "Comodato" },
        { "field": "tenure_type", "op": "eq", "value": "Alquiler" }
      ]
    }
  ]
}
```

### 3b.6 Evaluación cliente vs. servidor

**Cliente:** El motor emite un bloque `<script>` inline con un evaluador reactivo que
escucha cambios en los campos referenciados en condiciones `visible_when` y muestra u
oculta los elementos afectados sin round-trip al servidor. Los elementos se muestran/ocultan
con `display:none` — no se eliminan del DOM para preservar el estado del formulario.

**Servidor:** Al recibir el POST, el backend evalúa las mismas condiciones usando los
valores enviados. Los campos cuya condición no se cumple son ignorados en la validación y
en la persistencia, equivalente al comportamiento `if/then` de queryfy. Esto garantiza
consistencia aunque el usuario manipule el DOM manualmente.

### 3b.7 Interacción con campos requeridos

Si un campo tiene `visible_when` y aparece en `required`, la obligatoriedad sólo se aplica
cuando la condición de visibilidad se cumple. En el servidor, el campo se valida como
requerido sólo si la condición se evalúa como verdadera con los valores del request.

### 3b.8 Casos borde de visibilidad

**Campo referenciado en `visible_when` que no existe en el formulario:** El motor trata la
condición como no cumplida (elemento oculto). Se emite una advertencia en el log del servidor.

**Campo cuya condición depende de un campo oculto por permisos:** Si el campo referenciado
en `visible_when` no es visible para el usuario actual (oculto por permisos), el motor
evalúa la condición contra el valor persistido de ese campo (no contra un input del
formulario). Esto evita que campos ocultos por permisos provoquen visibilidad
indeterminada en otros campos.

**Tab cuyas secciones/filas quedan todas ocultas:** Si todas las secciones de un tab quedan
ocultas por visibilidad condicional o por permisos, el tab se omite de la barra de tabs.
Si el tab oculto era el tab activo, el motor activa el primer tab visible.

**Fila cuyas secciones quedan todas ocultas:** La fila se omite del HTML. No se emite el
contenedor de la fila.

**Sección cuya condición depende de un campo `repeatable`:** No soportado en el MVP. Las
condiciones `visible_when` sólo pueden referenciar campos escalares del formulario raíz.

---

## 3c. Control de Acceso y Permisos

### 3c.1 Separación de mecanismos

Existen tres mecanismos distintos que controlan qué ve y qué puede editar un usuario.
Son ortogonales y se mantienen estrictamente separados:

| Mecanismo | Origen | Dónde vive | Aplicado por |
|-----------|--------|------------|--------------|
| Visibilidad condicional | Valor de otro campo | Schema (`x-seam-visible-when`, `visible_when`) | Motor (JS cliente + queryfy servidor) |
| Sólo lectura por ciclo de vida | Estado de la entidad | Handler / FSM | Motor vía `RenderContext` |
| Control de acceso por rol | Rol del usuario | `FieldPermission` en xolu | Motor vía `RenderContext` |

**El schema nunca contiene lógica de permisos.** El schema define qué existe. Los permisos
definen qué puede ver y hacer cada rol. Esta separación es una línea de seguridad: si los
permisos estuvieran en el schema, un usuario con acceso a la API podría leer el schema
completo e inferir la existencia de campos que no debería conocer. En el modelo DIY, el
operador que edita sus propios schemas podría accidentalmente exponer campos sensibles.

### 3c.2 `RenderContext` extendido

El `RenderContext` es la estructura que el handler construye antes de llamar al motor.
Concentra toda la información de estado y permisos necesaria para el renderizado:

```go
// RenderContext describe el contexto completo de renderizado de un formulario.
type RenderContext struct {
    // Identificación del formulario
    Module string // "assets"
    Form   string // "asset"

    // Acción
    Action    string // URL de destino del form
    Method    string // "post" | "put" | "patch"
    IsPartial bool   // true = dentro de SeamModal o SeamModal2
    IsEdit    bool   // true = modo edición, false = modo creación
    EntityID   int64  // ID de la entidad en modo edición (para detail/embed/tray endpoints)
    EntityType string // tipo de entidad: "asset" | "sensor" | "custodio" | etc. (para tray)
    CSRFToken  string

    // Estado de la entidad — sólo lectura por ciclo de vida
    ReadOnly bool // true = todo el formulario es de sólo lectura

    // Permisos resueltos — calculados por permService antes de llamar al motor
    // Todos los elementos listados aquí se omiten del HTML completamente (server-side).
    HiddenFields   []string         // nombres de campos a omitir
    HiddenSections []string         // IDs de secciones a omitir
    HiddenRows     map[string][]int // tab.id -> índices de filas ocultas dentro de ese tab
    HiddenTabs     []string         // IDs de tabs a omitir

    // Elementos de sólo lectura (se renderizan pero sin inputs editables)
    ReadOnlyFields   []string
    ReadOnlySections []string
    ReadOnlyTabs     []string

    // Permisos de tray — calculados por permService; keyed por section.id
    // Valores: "hidden" | "read" | "preview" | "full"
    // Si una sección attachable no tiene entrada aquí, el tray no se renderiza.
    TrayPermissions map[string]string

    // Locale activo para resolución i18n
    Locale string
}
```

> **Corrección, 2026-07-28:** `HiddenRows` estaba declarado aquí como `[]int` en
> versiones anteriores de este documento, mientras que §3c.4 (`ResolvedPerms`,
> que existe específicamente para poblar este campo) siempre declaró la misma
> información como `map[string][]int`. Una lista plana no puede distinguir
> "ocultar la fila 2 en el tab A" de "ocultar la fila 2 en el tab B" — y el
> primer schema de ejemplo de este mismo documento (§3.1) es multi-tab. Se
> tomó la forma de mapa como autoritativa por ser la única bien definida para
> más de un tab, y se corrigió aquí antes de que un caller real (el módulo
> Administration, aún no existe) pudiera implementarse contra la versión
> plana incorrecta. Implementado en `internal/formengine` (stage 6).

### 3c.3 Modelo `FieldPermission`

Las reglas de permisos se almacenan en xolu por tenant. Cada regla especifica el acceso de
uno o más roles a un elemento del formulario:

```go
// FieldPermission define el nivel de acceso de un conjunto de roles
// a un elemento específico de un formulario.
type FieldPermission struct {
    ID       string   // UUID
    TenantID string

    Module  string   // "assets"
    Form    string   // "asset"
    Target  string   // nombre del campo, ID de sección, ID de tab, índice de fila
    Type    string   // "field" | "section" | "tab" | "row"

    Roles   []string // ["admin", "supervisor", "technician"]
    Access  string   // "editable" | "readonly" | "hidden"
}
```

**Modelo permissive by default:** si no existe ninguna regla de permiso para un elemento,
ese elemento es visible y editable para todos los roles. Las restricciones se declaran
explícitamente. Esto es apropiado para sistemas de gestión operativa donde la mayoría de
los usuarios tienen acceso pleno y las restricciones son excepciones.

Para tenants con requisitos de seguridad más estrictos, el modelo puede invertirse a
**deny by default** mediante un flag en la configuración del tenant. En ese caso, todos
los elementos son ocultos por defecto y los permisos deben declararse explícitamente para
cada rol.

### 3c.4 Resolución de permisos

El servicio de permisos resuelve las listas del `RenderContext` consultando las reglas
almacenadas en xolu para el tenant, módulo, formulario y roles del usuario:

```go
// PermService resuelve permisos para un usuario y formulario.
type PermService interface {
    Resolve(tenantID string, userRoles []string, module, form string) *ResolvedPerms
}

type ResolvedPerms struct {
    HiddenFields   []string
    HiddenSections []string
    HiddenTabs     []string
    HiddenRows     map[string][]int // tab_id → índices de filas ocultas

    ReadOnlyFields   []string
    ReadOnlySections []string
    ReadOnlyTabs     []string
}
```

La resolución sigue este orden para cada elemento:

```
1. Si existe regla con Access="hidden"   para alguno de los roles del usuario → hidden
2. Si existe regla con Access="readonly" para alguno de los roles del usuario → readonly
3. Si existe regla con Access="editable" para alguno de los roles del usuario → editable
4. Sin regla → editable (permissive default) | hidden (deny default)
```

Cuando un usuario tiene múltiples roles con reglas contradictorias, la regla más
restrictiva tiene precedencia: `hidden` > `readonly` > `editable`.

### 3c.5 Aplicación de permisos en el motor

El motor aplica los permisos del `RenderContext` **server-side**, omitiendo los elementos
del HTML generado. Un campo `hidden` no existe en el DOM — no puede ser enviado en el POST
ni manipulado en el cliente.

**Campos hidden:** no se emite el `<div>` del campo. El campo no existe en el formulario.

**Campos readonly:** se emite el valor como `<span>` o `<input disabled>`, no como input
editable. El campo tampoco se procesa al recibir el POST — cualquier valor enviado para
un campo readonly es ignorado por el backend.

**Secciones hidden:** no se emite la sección ni ninguno de sus campos. Los campos de la
sección se tratan como si fueran `HiddenFields` individualmente a efectos de validación.

**Secciones readonly:** todos los campos de la sección se renderizan como readonly.
Equivalente a agregar todos sus campos a `ReadOnlyFields`.

**Tabs hidden:** no se emite el botón del tab ni su panel. Todo el contenido del tab
(secciones, filas, details, embeds) se omite.

**Tabs readonly:** todos los campos del tab se renderizan como readonly. Los botones de
`embed` dentro del tab no se emiten.

### 3c.6 Casos borde de permisos

**Tab activo oculto por permisos:** Si el tab que debería activarse por defecto (el primero
del schema) está en `HiddenTabs`, el motor activa el primer tab visible. Si no hay ningún
tab visible, el formulario se renderiza vacío con un mensaje `core.errors.no_visible_tabs`.

**Todos los campos de un tab ocultos individualmente:** Si todos los campos de todos las
secciones de un tab están en `HiddenFields` (sin que el tab mismo esté en `HiddenTabs`),
el tab se omite igual que si estuviera en `HiddenTabs`. El motor verifica esta condición
antes de emitir el botón del tab.

**Fila cuyas secciones quedan todas ocultas por permisos:** La fila se omite. Si la fila
era la única de su tab, el tab también se omite.

**Campo requerido oculto por permisos:** Si un campo aparece en `required` del schema y
también en `HiddenFields`, el backend omite la validación de ese campo. Un campo que el
usuario no puede ver no puede ser obligatorio para ese usuario.

**Campo readonly con valor inválido pre-existente:** Si un campo readonly contiene un valor
que no pasaría validación (dato histórico corrupto), el motor lo muestra igualmente. Los
campos readonly no se validan al guardar — su valor no se modifica.

**Sección parcialmente oculta:** Si sólo algunos campos de una sección están en
`HiddenFields` (no la sección entera), la sección se muestra con los campos visibles
restantes. Si tras aplicar los campos ocultos la sección queda vacía, se aplica la regla
de sección vacía (omitir).

**`embed` en tab readonly:** Los botones de acción de embeds no se emiten en tabs readonly.
Los panels de `detail` (sólo lectura por naturaleza) sí se emiten aunque el tab sea readonly.

**Permisos sobre campos de sub-formulario `embed`:** Los permisos sobre campos de un
`embed` se resuelven con `Type="field"` y `Form="{form}.embed.{embed_id}"`. El sub-schema
del embed se renderiza con su propio contexto de permisos al abrir el modal.

**Visibilidad condicional sobre campo oculto por permisos:** Si el campo referenciado en
`visible_when` está oculto por permisos para el usuario actual, el motor evalúa la condición
contra el valor persistido de ese campo, no contra un input del formulario. Esto evita
visibilidad indeterminada. El resultado puede ser que un campo sea visible para el usuario
basándose en un valor que él mismo no puede ver — este es el comportamiento correcto y
esperado (el dato existe aunque no sea visible para ese rol).

### 3c.7 Administración de permisos desde el módulo Admin

El módulo Admin expone una UI para gestionar las reglas de `FieldPermission` por formulario:

**Vista de formulario:** listado de todos los elementos del formulario (tabs, secciones,
campos) con el nivel de acceso actual por rol. Filas con fondo destacado indican elementos
con restricciones activas.

**Editor de regla:** para cada elemento, un selector de roles y un selector de nivel de
acceso (`editable` / `readonly` / `hidden`). Los cambios se persisten inmediatamente en
xolu y se aplican en la siguiente carga del formulario (sin reinicio).

**Vista de rol:** vista inversa — dado un rol, mostrar qué elementos del formulario son
visibles, readonly u ocultos. Útil para auditoría de permisos.

**Herencia de permisos:** una regla sobre una sección tiene precedencia sobre las reglas
individuales de sus campos. Una regla sobre un tab tiene precedencia sobre sus secciones.
El editor muestra la regla efectiva para cada campo considerando la herencia, distinguiendo
entre regla directa y regla heredada.

### 3c.8 Modelo de permisos del tray de adjuntos

El tray de adjuntos tiene su propio modelo de permisos, independiente del modelo de
campos, secciones y tabs. Un usuario puede tener acceso completo a todos los campos de
una sección y aun así tener acceso restringido al tray de esa sección.

#### Niveles de acceso del tray

| Nivel | `Access` | Comportamiento |
|-------|----------|----------------|
| Oculto | `"hidden"` | El tray no se renderiza. El usuario no sabe que existen adjuntos. |
| Lectura | `"read"` | Los pills son visibles con título, tipo y thumbnail. No se puede abrir el contenido del archivo. Sin botón "Adjuntar". |
| Previsualización | `"preview"` | Igual que `"read"` más: click en pill abre el visor con el derivado de thumble (JPEG de página, forma de onda, frame de video). Sin descarga. Sin acceso al gestor documental. |
| Completo | `"full"` | Igual que `"preview"` más: descarga del archivo original desde seam-filer y acceso al gestor documental para vincular/desvincular documentos. |

La descarga (`"full"`) está gatada en el servidor en el proxy de seam-filer — no sólo
ocultando el botón en el HTML. Un usuario con nivel `"preview"` que construya la URL
de descarga manualmente recibirá 403 del proxy.

#### `FieldPermission` para trays

Las reglas de permiso de tray se almacenan en xolu como `FieldPermission` con
`Type: "tray"` y `Target` igual al `id` de la sección:

```go
FieldPermission{
    Module:  "assets",
    Form:    "asset",
    Target:  "custodia",      // id de la sección con attachable: true
    Type:    "tray",
    Roles:   []string{"operator"},
    Access:  "preview",       // "hidden" | "read" | "preview" | "full"
}
```

#### Resolución de TrayPermissions en el RenderContext

El `permService.Resolve()` evalúa las reglas de tray para cada sección attachable del
formulario y popula el mapa `RenderContext.TrayPermissions`:

```go
// Para cada sección con attachable: true en el schema:
// 1. Buscar FieldPermission con Type="tray", Target=section.id
// 2. Aplicar resolución de conflictos de roles (más restrictivo gana,
//    o prioridad de rol según §15.3)
// 3. Si no hay regla → nivel por defecto del tenant (permissive = "full",
//    deny = "hidden")
// 4. Almacenar en TrayPermissions[section.id]
```

El motor no renderiza el tray si `TrayPermissions[section.id]` es `"hidden"` o si la
clave no existe en el mapa.

#### Endpoint del tray

La URL del tray htmx sigue el patrón:

```
GET /api/v1/documents/tray/{entity_type}/{entity_id}/{section_id}
```

El handler de este endpoint verifica nuevamente el permiso del usuario antes de
renderizar — el nivel de acceso también controla qué botones se incluyen en el HTML
del tray (el botón "Adjuntar" sólo aparece con nivel `"full"`).

---

## 4. Extensión `x-seam-input`

Especifica el tipo de widget a renderizar para un campo. Cuando está ausente, el motor infiere
el widget a partir del tipo JSON y `format`.

### 4.1 Inferencia de widget por defecto

| Tipo JSON | Format | Widget inferido |
|-----------|--------|-----------------|
| `string` | — | `text` |
| `string` | `email` | `email` |
| `string` | `uri` | `url` |
| `string` | `date` | `date` |
| `string` | `date-time` | `datetime-local` |
| `string` | `time` | `time` |
| `string` | `tel` | `tel` |
| `string` | `decimal` | `decimal` (T-10) |
| `string` + `enum` | — | `select` |
| `integer` o `number` | — | `number` |
| `boolean` | — | `checkbox` |
| `array` | — | `repeatable` |

### 4.2 Valores de `x-seam-input`

#### `text`
Input de texto de una línea. Comportamiento estándar HTML `type="text"`.

```json
"nombre_corto": {
  "type": "string",
  "title": "Nombre Corto",
  "x-seam-input": "text",
  "minLength": 1
}
```

#### `textarea`
Área de texto multilínea. Complementado por `x-seam-rows` (número de filas, por defecto 3).

```json
"descripcion": {
  "type": "string",
  "title": "Descripción",
  "x-seam-input": "textarea",
  "x-seam-rows": 3
}
```

#### `select`
Menú desplegable. Las opciones se derivan de `enum` (estático) o de `x-seam-source`
(dinámico). Siempre incluye una opción vacía inicial ("— Seleccionar —").

```json
"tenure_type": {
  "type": "string",
  "title": "Calidad de Tenencia",
  "x-seam-input": "select",
  "enum": ["Propiedad", "Comodato", "Préstamo", "Alquiler"]
}
```

#### `checkbox`
Casilla de verificación. Renderizada como `<input type="checkbox">` con su label a la derecha.

```json
"estado": {
  "type": "boolean",
  "title": "Activo",
  "x-seam-input": "checkbox"
}
```

#### `toggle`
Interruptor toggle (switch). Semánticamente equivalente a `checkbox` pero con renderizado
visual de toggle pill (patrón `peer-checked` de Tailwind). Usado en Settings para
notificaciones y features.

```json
"email_alerts_enabled": {
  "type": "boolean",
  "title": "Alertas por Email",
  "x-seam-input": "toggle",
  "x-seam-help": "Recibe email para alertas críticas"
}
```

#### `color`
Selector de color nativo HTML (`type="color"`). El valor se almacena como hex (ej: `#6366f1`).

```json
"color": {
  "type": "string",
  "title": "Color",
  "x-seam-input": "color",
  "x-seam-help": "Usado para identificación visual en la UI",
  "x-seam-help-style": "inline"
}
```

#### `document-links`
Lista dinámica de enlaces a documentos. Cada fila contiene tres campos: `nombre` (texto),
`url` (URL), `tipo` (texto opcional). El operador puede agregar y eliminar filas.
Implementado sobre el patrón `DocumentLinksField` existente.

```json
"loan_links": {
  "type": "array",
  "title": "Links Comodato",
  "x-seam-input": "document-links",
  "items": {
    "type": "object",
    "properties": {
      "nombre": { "type": "string", "title": "Nombre" },
      "url":    { "type": "string", "format": "uri", "title": "URL" },
      "tipo":   { "type": "string", "title": "Tipo (opcional)" }
    }
  }
}
```

#### `repeatable`
Sub-formulario con filas dinámicas y campos heterogéneos. Más flexible que `document-links`:
cada fila puede contener cualquier combinación de widgets, incluyendo selects dinámicos y
campos numéricos. Implementado sobre el patrón `serviceConfigsField` existente.

```json
"service_configs": {
  "type": "array",
  "title": "Configuraciones de Service",
  "x-seam-input": "repeatable",
  "items": {
    "type": "object",
    "properties": {
      "medida": {
        "type": "string",
        "title": "Medida",
        "x-seam-input": "select",
        "enum": ["Kms", "Horas", "Usos"]
      },
      "umbral": {
        "type": "number",
        "title": "Umbral"
      },
      "alerta_id": {
        "type": "integer",
        "title": "Alerta",
        "x-seam-input": "select",
        "x-seam-source": "entity:alertas"
      }
    }
  }
}
```

#### `asset-picker`
Selector de asset del grafo. Widget especializado para el campo `parent_id` y cualquier
relación de contención o uso entre assets. Muestra un selector con búsqueda y preview
del árbol de contención. El tipo de arista se especifica con `x-seam-relation`.

**Corrección (2026-08-17):** este párrafo decía anteriormente que el widget estaba
"reservado para implementación futura" y que en el MVP se renderizaba como `number`.
Eso quedó obsoleto con T-19/T-72 — el widget está implementado: búsqueda en vivo vía
`GET /picker/search`, resolución del label de un valor ya asignado vía el mismo
endpoint con `id=`, y autocompletado client-side sobre autoComplete.js (vendorizado,
MIT — `web/static/vendor/autocomplete@10.2.10.min.js`). Implementación en
`internal/formengine/widgets.go`'s `renderAssetPicker`; backend en
`internal/ui/asset_picker_search.go`.

```json
"parent_id": {
  "type": "integer",
  "title": "Asset Padre",
  "x-seam-input": "asset-picker",
  "x-seam-relation": "contains",
  "x-seam-help": "Asset que contiene este activo en la jerarquía",
  "x-seam-help-style": "inline"
}
```

#### `any-picker`
Added in T-97 (2026-10). Per this document's convention (see the note before the
T-10 entries below) new entries are written in English.

`any-picker` is the generic counterpart of `asset-picker`: a hidden input carrying the
submitted id plus a visible live-search box (autoComplete.js), generic over
`x-seam-source: "entity:<name>"`, backed by the same `GET /picker/search`
(`internal/ui/asset_picker_search.go`). It is implemented by `renderAnyPicker`
(`internal/formengine/widgets.go`). `asset-picker` stays registered and unchanged; asset
forms keep using it.

Unlike `asset-picker`, `any-picker` is safe inside repeatable rows. Its markup is inert and
carries data attributes (`data-seam-picker`, `data-entity`, optional `data-selector`);
one shared script, `SeamPicker` in `web/static/js/app.js`, initialises every picker on
the page, and `SeamRepeatable.add` initialises the ones in rows added later. Nothing is
looked up by a per-widget id, so the ids that repeated rows repeat cannot make two
pickers collide. Inside repeatable rows use `any-picker`, not `asset-picker`.

An optional **Browse** button is enabled by `x-seam-selector` (the key of a server-side
Selector source, see `docs/SELECTOR_SPEC.md`). `SeamPicker.browse` opens the Selector
dialog in `SeamModal2` with a generated `target`; the dialog dispatches
`seam:selector-chosen` with `{key, id, label, target}`, and `SeamPicker` matches
`detail.target` to the picker and sets the hidden value and the search box by property
assignment (never as HTML).

Security: only schema-authored strings (entity source, selector key, chrome label) appear,
in attributes escaped by minty; a record's label only travels through the event; a current
value appears only in the hidden input's `value` attribute.

```json
"owner_id": {
  "type": "integer",
  "title": "Owner",
  "x-seam-input": "any-picker",
  "x-seam-source": "entity:users",
  "x-seam-selector": "users"
}
```

#### Indexed naming for `repeatable` (x-seam-naming)
The original repeatable convention is flat parallel arrays, `{field}_{item}[]`. It cannot
represent a repeatable inside a repeatable (which line belongs to which block) and cannot
host widgets that look elements up by id. A repeatable therefore uses **indexed names**,
for example `blocks[0][name]` and `blocks[0][lines][1][name]`, when any of these holds:
`"x-seam-naming": "indexed"`, it is nested inside an indexed repeatable, or one of its
items is itself a repeatable, an `any-picker`, an `asset-picker`, an `icon-picker`, a `gps`, a
`signature`, a `barcode`, or a `text`/`tel` with `x-seam-input-mask`. Every other repeatable
keeps the flat names, so existing forms and handlers are unchanged.

The widgets in that list carry no inline script: their markup is inert (`data-seam-picker`,
`data-seam-widget`, `data-seam-mask`) and one shared initialiser in `app.js` (`SeamPicker`,
`SeamWidgets`) finds each widget relative to its own root. Browsers do not run scripts
inserted with `innerHTML`, which is how a row is added, so a widget with its own script only
worked in the first, server-rendered row. Two widgets stay unsuitable inside a row: `image`
(needs a saved record and one owner field) and `asset-type-select` (reloads the whole form).

Rows are added from a pristine `<template>` rendered next to the rows (one blank row whose
names carry an index token, `__D0__` for the outer level, `__D1__` for the next), by
`SeamRepeatable.add` in `app.js`, which substitutes a fresh index and initialises the pickers and the other script-driven widgets.
Indices only grow (a per-container counter), so removing a row never renumbers the others
and DOM order is ascending index order. Adding after removing every row works because the
template always exists. On the server, `ParseIndexedRepeatable(form, field)` and
`ParseIndexedForm(form, fields)` rebuild the structure from the posted values (rows
ordered by numeric index, nested repeatables under their field name; a row whose first
field is empty is dropped at every level), in the shape the renderer takes back as a value.

#### Repeatable rows: cell labels and visuals (0.4.1-pre3)
Every cell of a repeatable row (flat or indexed) says what it is, the way the Identity
Fields of the Asset Type form do:
* text, number, textarea and any-picker cells get a **placeholder** and an `aria-label`
  with the cell's label; a select's empty option reads `- <label> -`; a checkbox already
  shows its title.
* the label is, in order: the item's `x-seam-placeholder`; the i18n key
  `repeatable_item.<repeatable>.<item>`; the key `repeatable_item.<item>`; the item's
  `title`; the item's name. The i18n keys are optional, so a schema with titles is
  already labelled in the schema's language.
* `x-seam-placeholder` also works outside rows (plain fields) and takes the text as
  written, for example `"x-seam-placeholder": "key (e.g. serial_no)"`.

Rows are tinted cards (inline styles, independent of the Tailwind build) and the add
button is outlined. A repeatable nested in another is indented under a coloured rule,
has its own heading (its label) and a more strongly tinted row, so its rows cannot be
mistaken for the rows of the repeatable that holds it.

#### Layout, buttons and highlight (0.4.1-pre6)
* `"x-seam-layout": "form"` on a repeatable makes each row a card with its cells in a
  responsive grid, each under a small label (the item's title or its i18n key; a checkbox
  keeps its own text, a nested repeatable its own heading). The default `row` is
  unchanged. A nested repeatable chooses its own layout.
* Add and remove use the same icon buttons as the Identity Fields: an outlined `add`
  button with its label, and the red `remove_circle_outline` icon (with a title and an
  accessible name), in both flat and indexed repeatables.
* `x-seam-browse` on an any-picker with a selector: `button` (outlined, icon and label,
  the default), `icon` (small, icon only, with title and aria-label) or `link` (the
  original text link).
* `x-seam-highlight`: the matched part of a result is highlighted again (the
  asset-picker had lost it when result labels were made safe against HTML). It is built
  with DOM nodes by `SeamHighlight` in `app.js`, never with `innerHTML`; case- and
  accent-insensitive. `color` is bold in the theme's primary colour, `background` a
  translucent marker that reads on light and dark surfaces, `none` plain text.
* An icon-picker with no icon chosen shows an empty slot where the icon will be (a dashed
  outline with a muted ellipsis, the icon's own size): `.fe-icon-preview:empty` in app.css.
* The icon-picker's search dropdown and its Browse grid are never visible together:
  opening the grid closes the dropdown, and while the grid is open typing filters the
  grid instead.
* Theming: the component styles live in `web/static/css/app.css` and take every colour
  from variables defined for light and dark, and edit fields set explicit text colours
  (`text-gray-900 dark:text-white`), so nothing is black text on a dark background.

#### `icon-picker`
Same inert-markup design as `any-picker` (initialised by `SeamPicker`, so it also works
inside repeatable rows) but over `GET /picker/icons?q=`, a case-insensitive search of the
Material Icons ligature names vendored with the app (the list `make check-icons`
audits). Names starting with the text come first; at most 30 results. The submitted
value is the icon name (`"home"`); the glyph is previewed beside the box, in a large size,
and stays visible once chosen (and when an existing value is edited). The **Browse** button
beside the box opens a scrollable grid of every icon (`GET /picker/icons?all=1`, cached by
the client, drawn 150 at a time as it is scrolled); typing in the box filters the grid, the
current icon is outlined, and choosing one fills the box and the preview and closes the grid.

```json
"icon": { "type": "string", "title": "Icon", "x-seam-input": "icon-picker" }
```

`entity:users` is served by `internal/ui/picker_users.go` through `auth.UserService`
(never raw OQL, so password hashes never leave the auth package). The live search and
the `id=` lookup both apply the Selector's visibility rule: the caller's tenant, active
users only and, by default, only roles at or below the caller's. A user the caller may
not see is "not found" in the lookup, so the existing value's label may stay blank for
a user of a higher role; the hidden id is preserved on submit.

---

The six widget entries below were added in T-10 (2026-08). Per this
document's own convention (§2.1), they are written in English.

#### `signature`
Canvas-based signature capture. Value is a base64-encoded PNG data URL,
stored in a hidden input the canvas writes to on every stroke. A "Clear"
button resets the canvas and empties the hidden input. Security note: an
existing value is placed in the hidden input's own `value="..."` attribute
using minty's normal automatic escaping — never interpolated into the
accompanying `<script>` block. Only the schema-authored field name goes
into the script itself, confirmed by an adversarial test using a
script-tag-breakout-shaped value.

```json
"customer_signature": {
  "type": "string",
  "title": "Customer Signature",
  "x-seam-input": "signature"
}
```

#### `decimal`
Exposes xolu's own `format: "decimal"` schema-layer fixed-point support
(xolu's `docs/JSON_SCHEMA.md`) — value travels as a JSON string (e.g.
`"10.99"`), not a JSON number, to avoid float64 round-trip precision loss.
Renders as `type="text"`, deliberately never `type="number"`: an HTML
number input is float-backed and can silently coerce or reformat a value
like `"10.999"` in browser-dependent ways, exactly what the string wire
format exists to avoid. `inputmode="decimal"` gets the numeric keyboard on
mobile; a `pattern` attribute is generated from `decimalScale` (not
hand-authored per field, so it can't drift from the schema's own declared
scale). `decimalPrecision`/`decimalScale` are plain (non-`x-seam-`) JSON
Schema properties, matching xolu's own naming exactly; defaults (18/4) are
applied when either is omitted. Client-side pattern is a visual hint only —
xolu's own write path is the actual validation authority.

```json
"unit_price": {
  "type": "string",
  "title": "Unit Price",
  "format": "decimal",
  "decimalPrecision": 10,
  "decimalScale": 2,
  "x-seam-unit": "USD"
}
```

#### `multi-select`
Checkbox group, not a native `<select multiple>` — that control's
ctrl/cmd-click UX is not discoverable and is widely considered an
accessibility dead end. Options resolve from either `enum` (static) or
`x-seam-source` (dynamic), the same two branches `select` supports.
Submission shape matches `document-links`/`repeatable`'s own array
convention: repeated form keys (`fieldname[]`), one per checked option — not
a single delimited string.

```json
"applicable_regions": {
  "type": "array",
  "title": "Applicable Regions",
  "x-seam-input": "multi-select",
  "enum": ["north", "south", "east", "west"]
}
```

#### `barcode`
Text field for a linear EAN/UPC barcode value, with a live SVG preview
rendered via JsBarcode (vendored, MIT —
`web/static/vendor/jsbarcode-ean-upc@3.12.3.min.js`, the EAN/UPC-only build,
not the full all-symbologies bundle). Distinct from the QR support
`internal/qrcode` already has — QR is a 2D symbology, EAN/UPC is 1D. The
submitted field value is the plain code string itself; the barcode image is
a live visual aid for catching a mistyped digit before submitting, not a
separately-encoded value. `x-seam-barcode-format` selects the symbology
(`EAN13` default, or `EAN8`/`UPC`/`UPCE`). This widget generates and
displays a barcode from typed input — it does not scan one via a device
camera, a different, harder feature not attempted here.

```json
"product_code": {
  "type": "string",
  "title": "Product Code",
  "x-seam-input": "barcode",
  "x-seam-barcode-format": "EAN13"
}
```

#### `gps`
Device-location capture plus a Leaflet map picker (reuses the same
vendored Leaflet already in production for the Map page — no new
vendoring needed). Value is a single `"lat,lng"` string (e.g.
`"-34.9011,-56.1645"`), editable directly in the text input or by
clicking/dragging the marker on the map. Deliberately a single
new field, not a migration of any existing separate latitude/longitude
number fields on a given entity — that is a schema-content decision, left
to whoever adopts this widget for their own schema.

The field is the text input plus a small pin button. The map lives in a dropdown
panel under that button (the same pattern as the icon-picker's grid), aligned to the
button's right edge, built the first time the panel opens, so a page with many `gps`
fields (or a repeatable with a `gps` cell) pays nothing for maps nobody opens. The panel
holds an icon-only "use my location" button (browser Geolocation API), the map, an
expand button and a Done button; Escape or a click outside also closes it. If the
right-aligned panel would leave the screen on the left (a narrow cell near the left
edge), it is anchored to the left edge instead.

The panel has two modes, like the icon-picker's two ways of opening: the dropdown above
(the default), and `wide`, the full width of the field in the page flow, which pushes
what follows down. Inside a repeatable's row the field is one narrow cell, so while a wide
panel is open its cell takes the whole row (form layout: every grid column; row layout: a
line of its own). The expand button switches between them at any time, and
`"x-seam-panel": "wide"` on the field makes it start wide.

The pin is an inline SVG in a Leaflet
`divIcon`: Leaflet's default marker builds its image URL from a detected path,
which resolved to a doubled `/static/vendor/images//static/vendor/images/...`
and 404ed, so the widget references no marker image at all.

The markup is inert (`data-seam-widget="gps"`, `data-gps-toggle`, `data-gps-panel`,
`data-gps-map`, `data-gps-locate`, `data-gps-close`) and `SeamWidgets` in `app.js`
initialises each widget relative to its own root, so any number of `gps` fields,
including those in repeatable rows, work independently.

```json
"gps_location": {
  "type": "string",
  "title": "GPS Location",
  "x-seam-input": "gps"
}
```

#### `image`
Photo gallery backed by xolu `/blob` storage plus a metadata entity
(`seam_media_items`: owner type/id/field, blob key, size, position,
uploaded-by/at) for hierarchy and lookup — a deliberately separate
abstraction from `internal/media` (the future contract for a real
seam-filer/thumble/fijuggler stack, currently a stub), structured so a
future migration to route through that stack instead of blobs directly is
a backend swap behind the same {list, upload, delete} shape, not a widget
rewrite. Fronted by a Lit-wrapped Dropzone.js upload UI (vendored, MIT —
`web/static/vendor/dropzone@5.9.3.min.js`; Lit shell:
`web/static/js/file-upload.js`, `<seam-file-upload>`), with a real
drag-and-drop drop zone and per-file upload progress. Grouping is
`(owner_type, owner_id, owner_field)` — one gallery per schema field per
record, not one gallery per record; a schema with two `image` fields on
one entity gets two independent galleries.

Edit-mode only, matching §3.7's Details/Embeds precedent (a new,
unsaved record has no entity ID yet to attach images to) — but rendered as
an explicit "save the record first" message rather than silently omitted,
since this is a field-level widget (an empty, unexplained control under a
label would look broken) where Details/Embeds are tab-level (can simply
not exist).

Backend: `internal/ui/media_gallery.go` (`GalleryUpload`, `GalleryList`,
`GalleryDelete`, `GalleryBlobProxy`, routes under `/gallery/*`).
`owner_type` is checked against a small allowlist and `owner_field`
against a strict identifier pattern before either reaches an OQL query —
never interpolated from client input unchecked.

A field that a section title already names can hide its label with `"x-seam-hide-label": true`:
the label stays in the page for screen readers (visually hidden). The Assets form's `photos`
field does, so the section is titled once.

```json
"photos": {
  "type": "string",
  "title": "Photos",
  "x-seam-input": "image"
}
```

---

## 5. Extensión `x-seam-source`

Especifica la fuente de datos dinámica para widgets que requieren una lista de opciones
cargada en tiempo de ejecución (selects FK, asset-picker).

### 5.1 Formato

```json
"x-seam-source": "entity:<nombre-entidad>"
```

### 5.2 Entidades del sistema disponibles

| Valor | Entidad | Label en UI |
|-------|---------|-------------|
| `entity:marcas` | Maestro de marcas | `{codigo} — {nombre}` |
| `entity:modelos` | Maestro de modelos | `{nombre}` |
| `entity:regiones` | Maestro de regiones | `{descripcion}` |
| `entity:ubicaciones` | Maestro de ubicaciones | `{nombre_corto}` |
| `entity:custodios` | Maestro de custodios | `{nombre_razon_social}` |
| `entity:monedas` | Maestro de monedas | `{codigo_iso} {simbolo}` |
| `entity:centros-costo` | Maestro de centros de costo | `{codigo_contable} — {nombre}` |
| `entity:alertas` | Maestro de alertas | `{tipo} ({criticidad})` |
| `entity:asset-types` | Tipos de asset | `{name}` |
| `entity:assets` | Assets (para asset-picker) | `{name}` |
| `entity:users` | Usuarios visibles para quien consulta (solo any-picker, T-97) | `{name} ({email})` |
| `entity:document-types` | Tipos de documento (módulo documents) | `{name}` |

### 5.3 Opción vacía

El motor siempre incluye una opción vacía inicial para selects con `x-seam-source`. El
texto de esta opción es "— Sin asignar —" para selects FK y "— Seleccionar —" para selects
de enumeración. El campo se considera no obligatorio si no aparece en `required`.

### 5.4 Opción nula vs. requerido

Un select con `x-seam-source` que aparezca en el array `required` del schema no incluye
opción vacía y el motor añade el atributo `required` al `<select>` HTML resultante.

---

## 6. Extensión `x-seam-help`

Asocia un texto de ayuda al campo. El texto se muestra según el estilo especificado en
`x-seam-help-style`.

### 6.1 Propiedades

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `x-seam-help` | string | Texto de ayuda. Puede incluir HTML básico (`<code>`, `<strong>`). |
| `x-seam-help-style` | string | `"inline"` (por defecto) o `"tooltip"`. |

### 6.2 Estilo `inline`

El texto se renderiza debajo del widget como un párrafo `<p>` en `text-xs text-gray-500`.
Implementado sobre `FormFieldWithHelp` existente. Compatible con `aria-describedby`.

```json
"slug": {
  "type": "string",
  "title": "Slug",
  "x-seam-input": "text",
  "pattern": "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
  "x-seam-help": "Solo letras minúsculas, números y guiones",
  "x-seam-help-style": "inline"
}
```

Renderiza como:
```html
<div class="mb-4">
  <label for="slug">Slug</label>
  <input type="text" name="slug" pattern="^[a-z0-9][a-z0-9-]*[a-z0-9]$" aria-describedby="slug-help">
  <p id="slug-help" class="mt-1 text-xs text-gray-500">Solo letras minúsculas, números y guiones</p>
</div>
```

### 6.3 Estilo `tooltip`

El texto se muestra en una burbuja flotante al hacer hover sobre un ícono `?` junto al
label. Este estilo requiere un nuevo componente JavaScript `SeamTooltip` en `app.js`,
no implementado en el MVP. Durante el MVP, `tooltip` se comporta igual que `inline`.

### 6.4 Interacción con `toggle`

Para campos `toggle`, el texto de `x-seam-help` se renderiza como la descripción visible
debajo del label del toggle (patrón `toggleField` de Settings), no como texto separado.

---

## 7. Extensión `x-seam-fk-popover`

Habilita el botón de icono `info` junto al label de un select FK. Al hacer clic, dispara
`SeamPopover.show(this, fieldName)` para mostrar un preview de la entidad actualmente
seleccionada con un enlace de navegación a su página de detalle.

### 7.1 Formato

```json
"x-seam-fk-popover": "/ruta-base-entidad"
```

### 7.2 Comportamiento

El botón de popover sólo aparece cuando hay un valor seleccionado (ID > 0). Al cambiar la
selección a "— Sin asignar —", el botón se oculta via `display:none`. El atributo
`data-jump-base` en el `<select>` permite que `initFKJumps()` en `app.js` actualice el
href del popover en tiempo real al cambiar la selección.

### 7.3 Campos que usan `x-seam-fk-popover` en el MVP

| Campo | URL base |
|-------|----------|
| `brand_id` | `/marcas` |
| `model_id` | `/modelos` |
| `location_id` | `/ubicaciones` |
| `custodian_id` | `/custodios` |
| `cost_center_id` | `/centros-costo` |
| `type_id` (asset type) | `/asset-types` |

### 7.4 Ejemplo

```json
"brand_id": {
  "type": "integer",
  "title": "Marca",
  "x-seam-input": "select",
  "x-seam-source": "entity:marcas",
  "x-seam-fk-popover": "/marcas"
}
```

---

## 8. Extensión `x-seam-relation`

Especifica el tipo de arista del grafo de assets para el widget `asset-picker`. Sólo es
relevante cuando `x-seam-input` es `asset-picker`.

### 8.1 Valores

| Valor | Semántica | Multiplicidad |
|-------|-----------|---------------|
| `contains` | El asset seleccionado es el padre contenedor de este asset. | 1 padre |
| `uses` | Este asset usa (comparte) el asset seleccionado. | N padres |

### 8.2 Contexto del modelo de assets

El modelo de grafo de Seam AMS define tres tipos de aristas:

- `contains` — jerarquía de contención primaria, 1 padre
- `uses` — relación de uso compartido, N padres
- `capacity` — relación entre Asset y Sensor, gestionada automáticamente

El widget `asset-picker` con `x-seam-relation: "contains"` permite al operador seleccionar
el asset padre en la jerarquía de contención. Con `"uses"` permite declarar recursos
compartidos (ej: módulo de telemetría usado por múltiples subsistemas, estación de bombeo
compartida entre dos líneas de tratamiento).

### 8.3 Nota de implementación

En el MVP, `asset-picker` se renderiza como campo numérico con nota visual. La implementación
completa requiere un componente UI de búsqueda y navegación del árbol de assets, que está
fuera del alcance del MVP del motor de formularios.

---

## 9. Validación y queryfy

### 9.1 Compilación del schema

Al cargar un tipo de asset o sensor, el backend compila el JSON Schema usando
`queryfy/builders/jsonschema.FromJSON()`. El schema compilado se cachea en memoria.
La validación de formularios usa el schema compilado directamente — no se recompila
en cada request.

### 9.2 Propiedades estándar reconocidas por queryfy

Las siguientes propiedades del schema son procesadas automáticamente por queryfy:

| Propiedad JSON Schema | Efecto en validación |
|----------------------|---------------------|
| `required` | Campo obligatorio. Error: "field is required" |
| `minLength` | Longitud mínima de string. |
| `maxLength` | Longitud máxima de string. |
| `pattern` | Validación de patrón regex. |
| `minimum` | Valor mínimo numérico. |
| `maximum` | Valor máximo numérico. |
| `format: "email"` | Validación de formato email. |
| `format: "uri"` | Validación de formato URL. |
| `format: "date"` | Validación de formato fecha (YYYY-MM-DD). |
| `format: "date-time"` | Validación de formato fecha-hora ISO 8601. |
| `format: "time"` | Validación de formato hora. |
| `format: "tel"` | Sin validación de formato propia de queryfy — reconocido a nivel de inferencia de widget (§4.1) únicamente. |
| `enum` | El valor debe pertenecer al conjunto. |

**Corrección (2026-08-17):** esta tabla no incluía `date-time`, `time`, ni `tel` pese a
que los tres ya estaban implementados (§4.1) y en uso (schema de ejemplo, §12). Corregido
al agregar `x-seam-*` documentation for T-10, que reveló la brecha al agregar
`format: "decimal"` (ver §9.3 sobre este formato específicamente).

### 9.3 Propiedades `x-seam-*` ignoradas por queryfy

Las extensiones `x-seam-*` son transparentes para queryfy — se almacenan como metadatos
desconocidos via `StoreUnknown: true` en las opciones de conversión y no afectan la
validación. El motor de formularios las lee directamente del schema raw almacenado en xolu.

**`format: "decimal"` is a different case, worth documenting precisely (found
2026-08-17 building the `decimal` widget, T-10).** Unlike an `x-seam-*`
property, `format` is a *standard* JSON Schema keyword — but `"decimal"` is
not a format queryfy recognises (it's a xolu-layer extension, not a JSON
Schema one; see xolu's own `docs/JSON_SCHEMA.md`). `StoreUnknown: true`
still handles it, but queryfy's own conversion returns this as a
**non-fatal warning** (`ConversionError.IsWarning == true`), not silent
success. Before T-10, `ParseSchema` treated *any* non-empty error list from
`jsonschema.FromJSON` as fatal — including warnings — because every format
string used anywhere in this codebase before `"decimal"` happened to be one
queryfy already recognises (the table in §9.2 above), so the
warning-vs-error distinction had never been exercised. This meant `format:
"decimal"` failed schema compilation outright until fixed: `ParseSchema`
now only fails on entries where `IsWarning` is false. Confirmed by a
regression test (`internal/formengine/schema_test.go`,
`TestParseSchema_DecimalDefaults`) that failed against the pre-fix code for
exactly this reason before the fix, not hypothetically.

### 9.4 Obligatoriedad condicional

El modelo de assets define tres niveles de obligatoriedad: `requerido-siempre`,
`requerido-condicional`, y `opcional`. Los campos `requerido-condicional` (ej: serial de
heladera obligatorio en régimen venta, opcional en alquiler) requieren validación
condicional que queryfy soporta a través de `DependentField` y validadores custom.

En el schema, la obligatoriedad condicional se expresa con la propiedad estándar `if/then`
de JSON Schema draft-07, que queryfy traduce a reglas de validación condicional:

```json
"if": {
  "properties": { "regimen": { "const": "venta" } }
},
"then": {
  "required": ["serial_heladera"]
}
```

Esto está fuera del alcance del MVP del motor pero el schema lo soporta por ser JSON Schema
estándar — no requiere extensiones propias.

---

## 10. Comportamiento del Renderer

### 10.1 Algoritmo de renderizado

El renderer del motor de formularios ejecuta el siguiente algoritmo al recibir un schema:

```
PRE-RENDER: Resolución de permisos y contexto
  - Llamar permService.Resolve(tenantID, userRoles, module, form)
  - Construir RenderContext con listas HiddenFields, HiddenTabs, etc.
  - Si ReadOnly=true, agregar todos los campos a ReadOnlyFields

1. Leer x-seam-form del schema raíz
2. Determinar ancho del contenedor a partir de x-seam-form.width
3. Si x-seam-form.tabs existe:
   a. Para cada tab:
      — Si tab.id en HiddenTabs → omitir tab (no emitir botón ni panel)
      — Si todos los campos de todas las secciones del tab están en HiddenFields → omitir tab
   b. Renderizar barra de tabs con los tabs visibles
      — Si el primer tab fue omitido, activar el primer tab visible
      — Si no hay ningún tab visible, emitir mensaje core.errors.no_visible_tabs
   c. Para cada tab visible:
      i.  Determinar fuente de layout: tab.rows o tab.sections
      ii. Para cada fila (row):
          — Evaluar visible_when de la fila → si no se cumple, omitir fila
          — Si todas las secciones de la fila están ocultas → omitir fila
          — Calcular anchos (widths explícitos o reparto igual)
          — Emitir contenedor de fila (grid-cols-12)
          iii. Para cada sección en la fila:
               — Si sección.id en HiddenSections → omitir sección
               — Evaluar visible_when de la sección → si no se cumple, omitir sección
               — Resolver label de sección via i18n
               — Renderizar encabezado de sección (H3 uppercase)
               — Aplicar layout interno de sección (1-col / 2-col)
               — Para cada campo en sección.fields:
                 * Si campo en HiddenFields → omitir campo completamente
                 * Evaluar x-seam-visible-when del campo → si no se cumple, omitir campo
                 * Resolver label via i18n
                 * Determinar widget (x-seam-input o inferencia por tipo)
                 * Si campo en ReadOnlyFields o sección en ReadOnlySections o tab en
                   ReadOnlyTabs o RenderContext.ReadOnly=true → renderizar como readonly
                 * Renderizar widget con label, valor actual, error si existe
               — Si sección queda vacía tras aplicar ocultamientos → omitir sección
      iv. Si IsEdit y tab.details existe:
          Para cada panel en tab.details:
          — Renderizar contenedor con título (label resuelto por i18n)
          — Renderizar indicador htmx (hx-get=source, hx-trigger="revealed")
      v.  Si IsEdit y tab.embeds existe y tab NO está en ReadOnlyTabs:
          Para cada embed en tab.embeds:
          — Si embed.trigger == "button": renderizar botón disparador
          — Si embed.trigger == "inline": renderizar sub-formulario inline
      vi. Si IsEdit:
          Para cada sección en el tab con attachable: true:
          — Leer TrayPermissions[sección.id] del RenderContext
          — Si valor es "hidden" o clave ausente → omitir tray
          — Si valor es "read" | "preview" | "full":
            * Emitir contenedor de tray con hx-get y hx-trigger="revealed"
            * URL: /api/v1/documents/tray/{EntityType}/{EntityID}/{sección.id}
            * Pasar nivel de acceso como query param: ?access={nivel}
            * El endpoint renderiza el contenido del tray incluyendo sólo
              los controles correspondientes al nivel de acceso
4. Emitir bloque <script> del evaluador reactivo de visible_when
   — El script escucha cambios en todos los campos referenciados en condiciones
   — Al cambiar un valor, re-evalúa las condiciones y muestra/oculta los elementos
5. Si x-seam-form.tabs no existe:
   Renderizar campos en orden de schema.properties sin tabs ni secciones,
   aplicando HiddenFields, ReadOnlyFields y visible_when
6. Renderizar botones de acción según contexto (modal parcial vs. página completa)
   — Si RenderContext.ReadOnly=true: omitir botón de submit, mostrar sólo "Cerrar"
```

### 10.2 Contextos de renderizado

El motor soporta tres contextos de renderizado, equivalentes a los existentes:

| Contexto | Descripción | Botones |
|----------|-------------|---------|
| `full-page` | Página completa, centrada con max-width | Inline al final del form |
| `modal` | SeamModal full-size (Assets) | En `data-modal-footer`, fuera del body scrollable |
| `modal2` | SeamModal2 con ancho configurable (Maestros) | En `data-modal-footer` |

El contexto se determina por el flag `IsPartial` del handler, igual que en la implementación
actual.

### 10.3 Estado de errores

Los errores de validación de queryfy se propagan campo a campo. El renderer consulta el
mapa de errores `map[string]string` (clave = nombre del campo, valor = mensaje de error)
y aplica el estado de error al widget correspondiente:

- Borde rojo en el input (`border-red-500`)
- Mensaje de error debajo del widget (`text-xs text-red-600`)
- Banner de error general al tope del formulario si hay errores no asociados a un campo

### 10.4 Selector dinámico de tipo de entidad

El campo `tipo_id` (Tipo de Activo) tiene comportamiento especial: al cambiar la selección,
dispara una llamada htmx que carga los campos de identidad y estado específicos del tipo
seleccionado. Este es el widget `x-seam-input: "entity-type-select"` — una capacidad
general del motor, no una implementación específica de assets.

Cualquier módulo puede usar `"entity-type-select"` para cualquier campo cuyo cambio deba
cargar dinámicamente campos adicionales. La configuración especifica el target htmx
(`#asset-type-fields` en assets, pero configurable) y el endpoint que devuelve los campos
adicionales renderizados.

En el schema, se declara como `x-seam-input: "entity-type-select"` con un atributo
`x-seam-type-fields-target` que indica el id del contenedor htmx destino. El motor
emite el hx-get y hx-target correspondientes al renderizar el widget.

---

## 11. Schema Completo de Ejemplo — Custodio

El siguiente schema expresa el formulario de Custodio (el más rico de los maestros actuales)
en la forma que el motor esperaría. No usa tabs por ser un formulario simple.

```json
{
  "type": "object",
  "title": "Custodio",
  "properties": {
    "tipo_sujeto": {
      "type": "string",
      "title": "Tipo de Documento",
      "x-seam-input": "select",
      "enum": ["DNI", "CI", "RUT", "RUC"]
    },
    "nombre_razon_social": {
      "type": "string",
      "title": "Nombre / Razón Social"
    },
    "identificador_fiscal": {
      "type": "string",
      "title": "Identificador Fiscal"
    },
    "alias": {
      "type": "string",
      "title": "Alias (opcional)"
    },
    "email": {
      "type": "string",
      "title": "Email",
      "format": "email"
    },
    "telefono": {
      "type": "string",
      "title": "Teléfono (opcional)",
      "format": "tel"
    },
    "direccion": {
      "type": "string",
      "title": "Dirección (opcional)"
    },
    "estado": {
      "type": "boolean",
      "title": "Activo",
      "x-seam-input": "checkbox"
    }
  },
  "required": ["tipo_sujeto", "nombre_razon_social", "identificador_fiscal", "email"]
}
```

---

## 12. Schema Completo de Ejemplo — Asset (fragmento, Tab Principal)

> **Corrección, 2026-07-28:** este fragmento usaba `"nombre"` y `"tipo_id"`
> como nombres de propiedad para el nombre del asset y su tipo. El formulario
> estático real (`internal/ui/asset_detail.go`) usa `name` y `type_id`
> (inglés) para estos dos campos específicos — inconsistente con el resto
> del formulario, que sí usa nombres en español, pero confirmado exhaustivamente
> contra cada `mi.Name(...)` real del código, no asumido. Usar los nombres en
> español habría producido un formulario dinámico que envía `nombre`/`tipo_id`
> en el POST mientras el handler real espera `name`/`type_id` — el campo
> nombre del asset habría llegado vacío en cada envío, silenciosamente.
> Corregido aquí y en `internal/formengine/schemas/asset/asset.json`
> (implementado en FE-22/FE-23). Los sub-campos `nombre` dentro de los
> ejemplos de `document-links` (§4.2, y los tres campos de enlaces en este
> mismo §12) son correctos tal cual están — son un campo distinto,
> propio de cada fila de enlace, no relacionado con el nombre del asset.

> **Segunda corrección, 2026-07-28:** el resto de las 40 propiedades de este
> fragmento usaban nombres en español (`criticidad`, `calidad_tenencia`,
> `custodio_id`, etc.), heredados del formulario estático real, que sí usa
> español de forma consistente salvo por `name`/`type_id` (corrección
> anterior). Decisión del equipo técnico: los identificadores de código —
> incluyendo los nombres de propiedad del schema — deben quedar en inglés
> en todos los casos; el soporte bilingüe (español e inglés) es
> responsabilidad exclusiva de la capa i18n (§4, T-13), no de los nombres
> de campo. Renombrado aquí y en `asset.json`. El formulario estático
> real sigue usando los nombres en español internamente — durante la
> coexistencia (§13.3), la capa de persistencia de FE-23 traduce entre
> los nombres en inglés del schema y las claves en español que xolu ya
> almacena, para que ambos formularios sigan operando sobre los mismos
> datos sin conflicto. Esa traducción es exclusiva de FE-23; no forma
> parte de este documento.

```json
{
  "type": "object",
  "title": "Asset",
  "x-seam-form": {
    "width": "2xl",
    "tabs": [
      {
        "id": "tab-principal",
        "label": "assets.forms.asset.tab.principal",
        "sections": [
          {
            "id": "basico",
            "label": "assets.forms.asset.section.basico",
            "layout": "2-col-left",
            "fields": ["name", "type_id", "status", "criticality", "connectivity_tier"]
          },
          {
            "id": "identificacion",
            "label": "assets.forms.asset.section.identificacion",
            "layout": "2-col-right",
            "fields": ["giai", "grai", "barcode_rfid", "technical_tag"]
          },
          {
            "id": "caracterizacion",
            "label": "assets.forms.asset.section.caracterizacion",
            "layout": "2-col-right",
            "fields": ["brand_id", "model_id", "serial_1", "serial_2", "lot"]
          }
        ]
      },
      {
        "id": "tab-loc",
        "label": "assets.forms.asset.tab.loc",
        "sections": [
          {
            "id": "ubicacion",
            "label": "assets.forms.asset.section.ubicacion",
            "layout": "2-col-left",
            "fields": ["location_id", "region_id", "latitude", "longitude", "parent_id"]
          },
          {
            "id": "custodia",
            "label": "assets.forms.asset.section.custodia",
            "layout": "2-col-right",
            "fields": ["custodian_id", "tenure_type", "loan_expiry", "loan_links"]
          }
        ]
      },
      {
        "id": "tab-ciclo",
        "label": "assets.forms.asset.tab.ciclo",
        "sections": [
          {
            "id": "fechas",
            "label": "assets.forms.asset.section.fechas",
            "layout": "2-col-left",
            "fields": [
              "purchase_date", "expected_arrival_date", "arrival_date",
              "obsolescence_date", "expiration_date", "decommission_date",
              "decommission_reason"
            ]
          },
          {
            "id": "docs-ciclo",
            "label": "assets.forms.asset.section.docs_ciclo",
            "layout": "2-col-right",
            "fields": ["lifecycle_links"]
          }
        ]
      },
      {
        "id": "tab-finserv",
        "label": "assets.forms.asset.tab.finserv",
        "sections": [
          {
            "id": "finanzas",
            "label": "assets.forms.asset.section.finanzas",
            "layout": "2-col-left",
            "fields": [
              "purchase_price", "currency_id", "current_price",
              "cost_center_id", "warranty_end", "document_links"
            ]
          },
          {
            "id": "service",
            "label": "assets.forms.asset.section.service",
            "layout": "2-col-right",
            "fields": [
              "temp_min", "temp_max",
              "refrigerant", "voltage_consumption",
              "service_configs"
            ]
          }
        ]
      }
    ]
  },
  "properties": {
    "name": {
      "type": "string",
      "title": "Nombre"
    },
    "type_id": {
      "type": "integer",
      "title": "Tipo de Activo",
      "x-seam-input": "asset-type-select",
      "x-seam-source": "entity:asset-types",
      "x-seam-fk-popover": "/asset-types"
    },
    "status": {
      "type": "string",
      "title": "Estado",
      "x-seam-input": "select",
      "enum": ["active", "silent", "alert", "lost"]
    },
    "criticality": {
      "type": "string",
      "title": "Criticidad",
      "x-seam-input": "select",
      "enum": ["Alta", "Media", "Baja"]
    },
    "connectivity_tier": {
      "type": "string",
      "title": "Conectividad",
      "x-seam-input": "select",
      "enum": ["qr", "gateway", "lora", "gps"]
    },
    "giai": { "type": "string", "title": "GIAI" },
    "grai": { "type": "string", "title": "GRAI" },
    "barcode_rfid": { "type": "string", "title": "Código Barras / RFID" },
    "technical_tag": { "type": "string", "title": "Tag Técnico" },
    "brand_id": {
      "type": "integer",
      "title": "Marca",
      "x-seam-input": "select",
      "x-seam-source": "entity:marcas",
      "x-seam-fk-popover": "/marcas"
    },
    "model_id": {
      "type": "integer",
      "title": "Modelo",
      "x-seam-input": "select",
      "x-seam-source": "entity:modelos",
      "x-seam-fk-popover": "/modelos"
    },
    "serial_1": { "type": "string", "title": "Serie 1" },
    "serial_2": { "type": "string", "title": "Serie 2" },
    "lot":    { "type": "string", "title": "Lote" },
    "location_id": {
      "type": "integer",
      "title": "Ubicación",
      "x-seam-input": "select",
      "x-seam-source": "entity:ubicaciones",
      "x-seam-fk-popover": "/ubicaciones"
    },
    "region_id": {
      "type": "integer",
      "title": "Región",
      "x-seam-input": "select",
      "x-seam-source": "entity:regiones"
    },
    "latitude":  { "type": "number", "title": "Latitud" },
    "longitude": { "type": "number", "title": "Longitud" },
    "parent_id": {
      "type": "integer",
      "title": "Asset Padre",
      "x-seam-input": "asset-picker",
      "x-seam-relation": "contains",
      "x-seam-help": "Asset que contiene este activo en la jerarquía",
      "x-seam-help-style": "inline"
    },
    "custodian_id": {
      "type": "integer",
      "title": "Custodio",
      "x-seam-input": "select",
      "x-seam-source": "entity:custodios",
      "x-seam-fk-popover": "/custodios"
    },
    "tenure_type": {
      "type": "string",
      "title": "Calidad de Tenencia",
      "x-seam-input": "select",
      "enum": ["Propiedad", "Comodato", "Préstamo", "Alquiler"]
    },
    "loan_expiry": { "type": "string", "title": "Vencimiento Comodato", "format": "date" },
    "loan_links": {
      "type": "array",
      "title": "Links Comodato",
      "x-seam-input": "document-links",
      "items": {
        "type": "object",
        "properties": {
          "name": { "type": "string", "title": "Nombre" },
          "url":    { "type": "string", "format": "uri", "title": "URL" },
          "type":   { "type": "string", "title": "Tipo (opcional)" }
        }
      }
    },
    "purchase_date":         { "type": "string", "title": "Fecha Compra",         "format": "date" },
    "expected_arrival_date":   { "type": "string", "title": "Fecha Prev. Ingreso",  "format": "date" },
    "arrival_date":        { "type": "string", "title": "Fecha Ingreso",         "format": "date" },
    "obsolescence_date":  { "type": "string", "title": "Fecha Obsolescencia",  "format": "date" },
    "expiration_date":     { "type": "string", "title": "Fecha Expiración",     "format": "date" },
    "decommission_date":           { "type": "string", "title": "Fecha de Baja",        "format": "date" },
    "decommission_reason": {
      "type": "string",
      "title": "Motivo de Baja",
      "x-seam-input": "select",
      "enum": ["Venta", "Desguace", "Robo", "Donación", "Otros"]
    },
    "lifecycle_links": {
      "type": "array",
      "title": "Links Ciclo de Vida",
      "x-seam-input": "document-links",
      "items": {
        "type": "object",
        "properties": {
          "name": { "type": "string", "title": "Nombre" },
          "url":    { "type": "string", "format": "uri", "title": "URL" },
          "type":   { "type": "string", "title": "Tipo (opcional)" }
        }
      }
    },
    "purchase_price":  { "type": "number", "title": "Precio Compra" },
    "currency_id": {
      "type": "integer",
      "title": "Moneda",
      "x-seam-input": "select",
      "x-seam-source": "entity:monedas"
    },
    "current_price": { "type": "number", "title": "Precio Actual" },
    "cost_center_id": {
      "type": "integer",
      "title": "Centro de Costo",
      "x-seam-input": "select",
      "x-seam-source": "entity:centros-costo",
      "x-seam-fk-popover": "/centros-costo"
    },
    "warranty_end": { "type": "string", "title": "Fin de Garantía", "format": "date" },
    "document_links": {
      "type": "array",
      "title": "Links Documentos",
      "x-seam-input": "document-links",
      "items": {
        "type": "object",
        "properties": {
          "name": { "type": "string", "title": "Nombre" },
          "url":    { "type": "string", "format": "uri", "title": "URL" },
          "type":   { "type": "string", "title": "Tipo (opcional)" }
        }
      }
    },
    "temp_min":  { "type": "number", "title": "Temp. Mín. (°C)" },
    "temp_max":  { "type": "number", "title": "Temp. Máx. (°C)" },
    "refrigerant":    { "type": "string", "title": "Refrigerante" },
    "voltage_consumption": { "type": "string", "title": "Voltaje / Consumo" },
    "service_configs": {
      "type": "array",
      "title": "Configuraciones de Service",
      "x-seam-input": "repeatable",
      "items": {
        "type": "object",
        "properties": {
          "measure": {
            "type": "string",
            "title": "Medida",
            "x-seam-input": "select",
            "enum": ["Kms", "Horas", "Usos"]
          },
          "threshold": { "type": "number", "title": "Umbral" },
          "alerta_id": {
            "type": "integer",
            "title": "Alerta",
            "x-seam-input": "select",
            "x-seam-source": "entity:alertas"
          }
        }
      }
    }
  },
  "required": ["name", "type_id", "criticality"]
}
```

---

> **Indicador de campo requerido, 2026-07-28:** el asterisco visual junto al
> label de un campo requerido es enteramente automático, derivado de que el
> nombre del campo esté en este array `required` -- nunca escrito a mano
> dentro del texto de `title` (ni literal ni como parte de una clave i18n
> resuelta). Antes de esta decisión, el fragmento de más arriba tenía
> `"Nombre *"`, `"Tipo de Activo *"`, `"Criticidad *"` como texto literal, y
> `region_id` también tenía `"Región *"` a pesar de no estar en este array
> -- una inconsistencia real del formulario estático original (FE-22).
> Escribir el asterisco a mano exige que cada paquete de locale recuerde
> agregarlo o quitarlo por campo, sin una única fuente de verdad contra la
> cual verificar -- este array `required` ya es esa fuente. `region_id` deja
> de mostrar el asterisco automáticamente al no estar aquí, sin necesidad de
> una decisión aislada sobre ese campo puntual. Implementado en
> `fkPopoverAwareLabel` (internal/formengine/renderer.go).

---

## 13. Consideraciones de Implementación

### 13.1 Estructura del paquete en el núcleo

El motor de formularios vive en el paquete compartido del núcleo de Seam AMS, accesible
por todos los módulos:

```
internal/
  formengine/
    engine.go       — punto de entrada: SchemaToForm(schema, values, errors) mi.H
    renderer.go     — renderizado de widgets individuales
    widgets.go      — implementaciones de cada tipo de widget
    schema.go       — lectura y resolución del JSON Schema + extensiones x-seam-*
    sources.go      — resolución de x-seam-source: carga de opciones desde entidades
```

### 13.2 Interfaz pública mínima

```go
// FormEngine renderiza un formulario dinámico a partir de un JSON Schema.
type FormEngine struct {
    Sources SourceResolver // resuelve entity:* → []Option
}

// Render produce el nodo minty equivalente al formulario.
func (e *FormEngine) Render(schema []byte, values map[string]interface{}, errors map[string]string, ctx RenderContext) mi.H
```

La definición completa y autoritativa de `RenderContext` está en §3c.2. La estructura
incluye todos los campos de permisos resueltos, EntityType, EntityID, TrayPermissions,
y el locale activo. No se duplica aquí para evitar deriva entre las dos definiciones.

### 13.3 Compatibilidad con formularios estáticos existentes

Durante la migración, los formularios estáticos y el motor dinámico coexisten. La migración
es progresiva: se convierte un formulario a la vez, comenzando por los maestros simples
(Región, Moneda, Marca) y avanzando hacia los más complejos (Asset, Sensor).

Los formularios estáticos existentes no se eliminan hasta que el motor dinámico haya
demostrado paridad funcional completa en producción.

### 13.4 Editor de schemas (fuera del MVP)

El editor visual de schemas (drag-and-drop de campos, configuración de tabs y secciones)
es un componente del módulo Admin, no del núcleo. Produce JSON Schema con extensiones
`x-seam-*` válido y lo persiste en xolu. El núcleo no tiene dependencia del editor — consume
el schema independientemente de cómo fue producido.

---

## 14. Tabla de Widgets — Resumen

| `x-seam-input` | Tipo JSON | Format | HTML generado | Estado MVP |
|-----------------|-----------|--------|---------------|------------|
| `text` (default) | `string` | — | `<input type="text">` | ✓ |
| `email` | `string` | `email` | `<input type="email">` | ✓ |
| `url` | `string` | `uri` | `<input type="url">` | ✓ |
| `tel` | `string` | `tel` | `<input type="tel">` | ✓ |
| `date` | `string` | `date` | `<input type="date">` | ✓ |
| `datetime-local` | `string` | `date-time` | `<input type="datetime-local">` | ✓ |
| `time` | `string` | `time` | `<input type="time">` | ✓ |
| `number` | `integer`/`number` | — | `<input type="number">` | ✓ |
| `select` (enum) | `string` + `enum` | — | `<select>` con opciones estáticas | ✓ |
| `select` (FK) | `integer` + source | — | `<select>` con opciones dinámicas | ✓ |
| `checkbox` | `boolean` | — | `<input type="checkbox">` | ✓ |
| `toggle` | `boolean` | — | Toggle pill (Tailwind peer-checked) | ✓ |
| `color` | `string` | — | `<input type="color">` | ✓ |
| `textarea` | `string` | — | `<textarea rows="N">` | ✓ |
| `document-links` | `array` | — | Lista dinámica nombre+url+tipo | ✓ |
| `repeatable` | `array` | — | Sub-formulario con filas dinámicas | ✓ |
| `asset-type-select` | `integer` | — | Select FK + htmx para campos dinámicos | ✓ |
| `asset-picker` | `integer` | — | Selector con búsqueda en vivo (autoComplete.js) | ✓ |
| `icon-picker` | `string` | — | Busqueda en vivo de iconos Material (`GET /picker/icons`); guarda el nombre del icono y muestra su vista previa (0.4.1-pre3) | ✓ |
| `any-picker` | `integer` | — | Igual que asset-picker, genérico por entidad; botón Browse opcional con `x-seam-selector` (T-97) | ✓ |
| `signature` | `string` | — | Canvas → PNG base64 (T-10) | ✓ |
| `decimal` | `string` | `decimal` | `<input type="text">` + pattern generado (T-10) | ✓ |
| `multi-select` | `array` | — | Checkbox group (T-10) | ✓ |
| `barcode` | `string` | — | Text input + SVG preview vía JsBarcode (T-10) | ✓ |
| `gps` | `string` | — | Text input + mapa Leaflet interactivo (T-10) | ✓ |
| `image` | `string` | — | Galería Dropzone.js + xolu `/blob` (T-10) | ✓ |
| `tooltip` help-style | — | — | Burbuja flotante en hover | ☐ Futuro |

---

## 15. Riesgos Conocidos y Decisiones Pendientes

Esta sección documenta riesgos de diseño identificados durante la especificación que
requieren una decisión explícita antes o durante la implementación.

---

### 15.1 Redundancia entre `rows` y `section_defs`

**Riesgo:** El modelo de layout de filas separa la definición de secciones (`section_defs`)
de su referencia en filas (`rows`). Si una sección se define en `section_defs` pero no
aparece en ninguna fila, se omite silenciosamente del formulario sin error ni advertencia.
Esto puede producir campos perdidos difíciles de diagnosticar.

**Opciones:**

A. El motor emite una advertencia en el log del servidor por cada `section_def` no
referenciada en ninguna fila. El formulario se renderiza igualmente con las secciones
referenciadas.

B. El motor rechaza el schema al compilarlo si hay secciones definidas pero no
referenciadas — error en tiempo de carga, no en tiempo de renderizado.

C. Colapsar las dos estructuras: la definición de la sección va inline dentro de la fila
que la contiene, eliminando la separación `rows`/`section_defs` completamente.

**Decisión:** Opción B. El parser de `x-seam-form` rechaza el schema en tiempo de carga
si encuentra `section_defs` con IDs que no aparecen en ninguna fila de `rows`.
El error es descriptivo: `"section_def 'custodia' defined but not referenced in any row"`.
Los schemas sin `rows` (que usan `sections` directamente) no tienen este problema —
sólo aplica cuando `section_defs` está presente.

---

### 15.2 Campos ocultos por visibilidad condicional y el POST

**Riesgo:** Los campos ocultos por `x-seam-visible-when` se renderizan con `display:none`
en el DOM para preservar el estado del formulario durante la sesión. Esto significa que
sus valores se incluyen en el POST cuando el usuario guarda. El servidor los ignora via
queryfy `if/then`, pero el comportamiento exacto del "ignorar" no está definido:

- **Silenciosamente descartado:** el valor llega pero no se persiste. No hay error. Si el
  campo tenía un valor previo en la base de datos, ese valor se preserva o se sobreescribe
  con el vacío según cómo se implemente el merge.
- **Activamente rechazado:** el servidor retorna un error si recibe un valor para un campo
  cuya condición de visibilidad no se cumple con los valores del mismo request.

Para la mayoría de campos el descarte silencioso es correcto. Para campos que representan
transiciones de estado, datos financieros, o cualquier valor con implicaciones externas
(webhooks, notificaciones), el rechazo activo es más seguro.

**Decisión:** política dual:

- **Por defecto:** descarte silencioso. El servidor ignora los valores recibidos para
  campos cuya condición de visibilidad no se cumple. El valor previo en la base de datos
  se preserva sin modificación (merge conservativo: si el campo no debería ser visible,
  su valor no se toca).
- **Rechazo activo:** un campo puede declarar `"x-seam-hidden-reject": true` en el
  schema para indicar que recibir un valor cuando está oculto es un error. El servidor
  retorna 422 con el mensaje de error correspondiente. Usar en campos que representan
  transiciones de estado irreversibles, confirmaciones financieras, o cualquier campo
  cuyo valor tiene efectos externos (webhooks, notificaciones).

El merge conservativo es implementable por queryfy: al procesar el PATCH body, los campos
en `HiddenFields` del `RenderContext` se excluyen del update antes de llegar a xolu.

---

### 15.3 Resolución de conflictos entre roles en permisos

**Riesgo:** El modelo `FieldPermission` especifica que cuando un usuario tiene múltiples
roles con reglas contradictorias, la regla más restrictiva tiene precedencia
(`hidden > readonly > editable`). Esto es seguro pero puede ser contra-intuitivo: un
usuario con roles `operator` (readonly en tab-finserv) y `manager` (editable en tab-finserv)
obtiene readonly, aunque `manager` sea el rol más privilegiado.

La alternativa — el rol más permisivo gana — es más intuitiva para la mayoría de los casos
pero introduce riesgo de escalada de privilegios no intencional si un usuario acumula roles
por error.

**Opciones:**

A. **Más restrictivo gana** (especificado actualmente): máxima seguridad, puede sorprender.
Requiere que los roles sean mutuamente excluyentes en la práctica, o que el administrador
sea muy cuidadoso al asignar roles.

B. **Más permisivo gana**: más intuitivo, mayor riesgo de escalada accidental. Requiere
auditoría de combinaciones de roles.

C. **Prioridad de rol explícita**: los roles tienen un orden de prioridad declarado
(`admin > manager > operator > viewer`). Cuando hay conflicto, el rol de mayor prioridad
gana. Más predecible que A y más seguro que B.

**Decisión:** Opción C — prioridad de rol explícita.

Los roles tienen prioridad numérica declarada. Cuando un usuario tiene múltiples roles
con reglas contradictorias para el mismo elemento, gana la regla del rol de mayor
prioridad (número más alto):

| Rol | Prioridad |
|-----|-----------|
| `admin` | 100 |
| `seam_consultant` | 90 |
| `manager` | 50 |
| `operator` | 30 |
| `viewer` | 10 |

`PermService.Resolve()` evalúa todas las reglas aplicables al usuario, las ordena por
prioridad de rol descendente, y toma la primera. Empates (mismo nivel de prioridad)
se resuelven con más-restrictivo-gana como desempate.

Las prioridades son configurables por tenant en una versión posterior. Para el MVP
están hardcodeadas en `PermService`.

---

### 15.4 `width: "fullscreen"` activa SeamModal en lugar de SeamModal2

**Riesgo de implementación:** Cuando `x-seam-form.width` es `"fullscreen"`, el motor
activa SeamModal (full-size) en lugar de SeamModal2 (posicionado con ancho configurable).
Este comportamiento no es obvio — el desarrollador que añade un schema con `width:
"fullscreen"` puede no saber que está cambiando el tipo de modal.

**Mitigación:** El motor debe emitir el atributo correcto de apertura modal (`SeamModal`
vs `SeamModal2`) basado en `width`, no en la elección del handler. El handler no debería
tener que saber cuál usar — lo delega al motor a partir del schema.

Esto requiere que el motor exponga una función auxiliar `ModalTriggerAttrs(schema) []mi.Attribute`
que el handler usa al construir el botón de apertura del modal:

```go
// En el handler, al construir el botón "Nuevo Asset":
attrs := engine.ModalTriggerAttrs(assetSchema, "/assets/new", "Nuevo Activo")
// → retorna hx-get, hx-target, onclick con SeamModal o SeamModal2 según schema.width
```

**Decisión:** confirmado. El motor expone `ModalTriggerAttrs(schema []byte, url, label string) []mi.Attribute`
que devuelve los atributos htmx y el onclick correcto para el modal adecuado según
`x-seam-form.width`. Los handlers nunca deciden directamente qué modal usar — lo
delegan al motor. Esto garantiza que cambiar el `width` en el schema actualiza
automáticamente el comportamiento de apertura sin tocar ningún handler.

---

### 15.5 Ciclo de vida del schema y deriva schema/datos

**Riesgo:** La especificación cubre el consumo de schemas almacenados en xolu pero no
define el ciclo de vida del schema: quién lo crea, cómo se valida, cómo se versiona, y
qué ocurre con los registros existentes cuando un schema cambia incompatiblemente.

Escenarios de cambio incompatible:

- Se elimina un campo del schema. Los registros existentes tienen ese campo en sus datos.
  ¿Se ignora silenciosamente? ¿Se muestra como dato huérfano? ¿Se bloquea la edición?
- Se añade un campo `required` a un schema existente. Los registros previos no tienen ese
  campo. ¿Se bloquea su edición hasta que se complete el campo? ¿Se permite guardar sin él?
- Se cambia el tipo de un campo (de `string` a `number`). Los datos existentes tienen
  strings que no parsean como números.

**Política mínima recomendada antes de la primera implementación:**

1. Cada schema tiene una versión (`schema_version: integer`).
2. Cada registro guarda la versión del schema con la que fue creado.
3. Los cambios **aditivos** (nuevo campo opcional) son compatibles — los registros viejos
   simplemente no tienen el campo.
4. Los cambios **de obligatoriedad** (campo opcional → requerido) son semi-compatibles —
   los registros viejos pueden leerse pero no guardarse hasta que el campo se complete.
5. Los cambios **destructivos** (eliminar campo, cambiar tipo) incrementan la versión
   mayor del schema. Los registros en versión anterior se muestran con el schema de su
   versión, en modo read-only, con un aviso de migración pendiente.

**Decisión:** adoptada la política de cinco puntos tal como se especifica arriba.
Esto es un prerequisito hard antes del primer despliegue de producción. El Admin module
(Schema Lifecycle, §6 de ADMIN_MODULE_SPEC) implementa el checker de compatibilidad y
el workflow draft/publish/rollback que hace esta política operativa.

---

### 15.6 Configuración avanzada del tray en secciones

**Riesgo:** La propiedad `attachable: true` en una sección es un toggle binario. No
expone configuración sobre el comportamiento del tray: cuántos pills mostrar antes del
"Ver todos", qué tipos de entidad pueden ser vinculados como adjuntos desde esta sección,
o si el tray permite vincular documentos de cualquier tipo o sólo de ciertos tipos.

Para el MVP, `attachable: true` es suficiente. El comportamiento del tray es uniforme
en todas las secciones attachable. Las restricciones por tipo de documento o límite de
pills son razonables para una versión posterior.

**Decisión pendiente:** cuando se necesite configuración por sección, reemplazar el
boolean por un objeto:

```json
{
  "id":         "custodia",
  "attachable": {
    "max_pills":        8,
    "document_types":   ["factura", "acta", "certificado"],
    "entity_types":     ["asset", "sensor"]
  }
}
```

Hasta esa versión, el boolean se acepta como equivalente a `{ "max_pills": 8 }` con
todos los tipos de documento y entidad permitidos.

**Decisión MVP:** boolean. El upgrade path al objeto está documentado y es no-breaking
— el parser acepta ambas formas simultáneamente cuando se implemente el objeto.

---

*Documento producido en sesión de diseño de arquitectura de Seam AMS, Mayo 2026.*  
*Copyright (c) 2026 haitch — h@ual.li*
