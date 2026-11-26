# Seam AMS — Sistema de Internacionalización
## Especificación de Diseño

**Versión:** 0.1  
**Fecha:** Mayo 2026  
**Estado:** Borrador interno  
**Repositorio:** github.com/ha1tch/seam (núcleo compartido entre módulos)

---

## 1. Contexto y Motivación

### 1.1 Estado actual

El sistema i18n actual de Seam AMS usa un `Translator` con archivos JSON planos por locale
(`en.json`, `es.json`, `pt.json`) con claves de formato `modulo.entidad.campo`. Esta
implementación es funcional para una aplicación monolítica pero presenta limitaciones
críticas para el modelo modular y la administración DIY:

- Las claves son un espacio plano — no hay jerarquía ni agrupación real
- No hay metadatos por locale (dirección de texto, características especiales, iconos)
- Los archivos JSON son editados directamente — no existe interfaz administrativa
- El motor de formularios dinámico no tiene forma de resolver claves de traducción
  para labels generados en tiempo de ejecución a partir de schemas
- No hay soporte para pluralización, género gramatical ni interpolación estructurada
- La adición de un módulo nuevo requiere editar los archivos de todos los locales

### 1.2 Objetivos del nuevo sistema

El nuevo sistema i18n debe satisfacer simultáneamente cuatro requisitos:

**Técnico:** Integración natural con el motor de formularios, el sistema modular, y la
arquitectura de schemas JSON. Los labels de campos, mensajes de error, y textos de UI
deben ser resolubles a partir de claves estructuradas sin acoplamiento al código fuente.

**Operativo:** Los paquetes de texto deben poder administrarse desde la interfaz del módulo
Admin sin editar archivos ni recompilar. Un operador debe poder traducir o ajustar textos
de su instancia sin intervención de ingeniería.

**Modular:** Cada módulo de Seam AMS declara su propio namespace de traducción. El núcleo
define los namespaces compartidos. Los módulos no pueden acceder a los namespaces de otros
módulos directamente — sólo al núcleo y a los suyos propios.

**Internacional:** Los paquetes de locale llevan metadatos completos que permiten al renderer
adaptar el layout (RTL/LTR), elegir iconos SVG apropiados, ajustar formatos de fecha,
número y moneda, y aplicar reglas tipográficas específicas del idioma.

---

## 2. Modelo de Datos del Paquete de Locale

Cada locale es un **paquete** (`LocalePackage`) que combina metadatos del idioma con el
catálogo de traducciones organizado jerárquicamente.

### 2.1 Estructura del paquete

```json
{
  "meta": { ... },
  "modules": {
    "core": { ... },
    "assets": { ... },
    "admin": { ... },
    "monitoring": { ... },
    "reports": { ... }
  }
}
```

### 2.2 Sección `meta` — Metadatos del locale

```json
{
  "meta": {
    "code":        "ar",
    "name":        "العربية",
    "name_en":     "Arabic",
    "name_native": "العربية",
    "region":      "SA",
    "direction":   "rtl",
    "script":      "Arab",

    "typography": {
      "font_family":       "Noto Sans Arabic, sans-serif",
      "font_size_adjust":   1.1,
      "line_height_adjust": 1.2,
      "word_spacing":       "normal",
      "letter_spacing":     "normal"
    },

    "layout": {
      "direction":       "rtl",
      "text_align":      "right",
      "icon_flip":       true,
      "modal_placement": "right",
      "nav_side":        "right"
    },

    "formats": {
      "date":             "DD/MM/YYYY",
      "datetime":         "DD/MM/YYYY HH:mm",
      "time":             "HH:mm",
      "number_decimal":   "٫",
      "number_thousands": "٬",
      "currency_position": "after",
      "currency_space":    true,
      "first_day_of_week": 6
    },

    "icons": {
      "use_set":      "seam-ar",
      "fallback_set": "material",
      "overrides": {
        "nav.assets":   "inventory_ar.svg",
        "nav.sensors":  "sensors_ar.svg",
        "action.save":  "save_ar.svg"
      }
    },

    "input": {
      "numeral_system":    "arab-indic",
      "keyboard_hint":     "arabic",
      "autocomplete_lang": "ar"
    },

    "pluralization": {
      "rule": "arabic",
      "forms": ["zero", "one", "two", "few", "many", "other"]
    },

    "collation":   "ar-SA",
    "hyphenation": false,
    "active":      true,
    "rtl":         true,

    "notes": "Arabic (Modern Standard). Numerals may display as Eastern Arabic (٠١٢...) in native mode."
  }
}
```

### 2.3 Propiedades de `meta` — Referencia completa

#### Identidad

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `code` | string | Código BCP 47 del locale. Ejemplos: `"es"`, `"ar"`, `"he"`, `"zh-TW"`. |
| `name` | string | Nombre del idioma en ese mismo idioma. |
| `name_en` | string | Nombre del idioma en inglés. |
| `name_native` | string | Nombre del idioma en su escritura nativa (puede diferir de `name` para locales con múltiples escrituras). |
| `region` | string | Código de región ISO 3166-1 alpha-2 de la variante principal. |
| `script` | string | Código ISO 15924 de la escritura. Ejemplos: `"Latn"`, `"Arab"`, `"Hebr"`, `"Hans"`, `"Deva"`. |

#### Dirección y layout

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `direction` | string | `"ltr"` o `"rtl"`. Determina el atributo `dir` del `<html>`. |
| `layout.direction` | string | Igual que `direction`. Usado explícitamente por el renderer. |
| `layout.text_align` | string | `"left"`, `"right"`, o `"start"` (CSS logical). Valor por defecto del alineado de texto. |
| `layout.icon_flip` | boolean | `true` = los iconos direccionales (flechas, chevrons, back/forward) se voltean horizontalmente en RTL. |
| `layout.modal_placement` | string | `"right"` o `"left"`. Lado desde el que aparecen los modales en RTL. |
| `layout.nav_side` | string | `"left"` o `"right"`. Lado del panel de navegación lateral. |

#### Tipografía

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `typography.font_family` | string | Familia tipográfica recomendada. Incluida en el `<head>` cuando el locale está activo. |
| `typography.font_size_adjust` | number | Factor de escala sobre el tamaño base. `1.0` = sin ajuste. Útil para escrituras con proporciones distintas (árabe, devanagari). |
| `typography.line_height_adjust` | number | Factor de escala sobre la altura de línea base. |
| `typography.word_spacing` | string | Valor CSS para `word-spacing`. `"normal"` en la mayoría de los casos. |
| `typography.letter_spacing` | string | Valor CSS para `letter-spacing`. Algunos idiomas (árabe, persa) requieren `"normal"` explícito para evitar separación incorrecta de glifos. |

#### Formatos regionales

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `formats.date` | string | Patrón de fecha para visualización. Tokens: `DD`, `MM`, `YYYY`. |
| `formats.datetime` | string | Patrón de fecha y hora. |
| `formats.time` | string | Patrón de hora. |
| `formats.number_decimal` | string | Separador decimal. `"."` (inglés), `","` (español/europeo), `"٫"` (árabe-índico). |
| `formats.number_thousands` | string | Separador de miles. `","`, `"."`, `" "`, `"٬"`. |
| `formats.currency_position` | string | `"before"` o `"after"`. Posición del símbolo de moneda respecto al monto. |
| `formats.currency_space` | boolean | `true` = espacio entre símbolo y monto. |
| `formats.first_day_of_week` | integer | `0` = domingo, `1` = lunes, `6` = sábado. Afecta calendarios y selectores de fecha. |

#### Iconos

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `icons.use_set` | string | Nombre del conjunto de iconos SVG activo para este locale. `"material"` es el conjunto base. |
| `icons.fallback_set` | string | Conjunto a usar cuando `use_set` no contiene el icono solicitado. |
| `icons.overrides` | object | Mapa de clave de icono → nombre de archivo SVG. Permite reemplazar iconos individuales por variantes culturalmente apropiadas. Por ejemplo, la luna creciente (`☽`) puede ser preferida sobre un sol (`☀`) en ciertos contextos para locales de países de mayoría musulmana. |

#### Input y teclado

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `input.numeral_system` | string | `"latn"` (0-9), `"arab-indic"` (٠-٩), `"devanagari"` (०-९), etc. Afecta la visualización de números en inputs nativos. |
| `input.keyboard_hint` | string | Valor del atributo `inputmode` para campos de texto en dispositivos móviles. |
| `input.autocomplete_lang` | string | Código de idioma para el atributo `lang` en inputs de texto libre, usado por el autocompletado del browser. |

#### Pluralización

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `pluralization.rule` | string | Nombre de la regla de pluralización CLDR. Valores: `"one-other"` (inglés, español), `"arabic"` (árabe, con 6 formas), `"slavic"` (ruso, polaco), `"east-slavic"`, `"romanian"`, `"none"` (japonés, chino). |
| `pluralization.forms` | array | Lista de formas disponibles para este locale. Subconjunto de `["zero","one","two","few","many","other"]`. |

#### Otras propiedades

| Propiedad | Tipo | Descripción |
|-----------|------|-------------|
| `collation` | string | Código de colación BCP 47 para ordenamiento alfabético correcto. |
| `hyphenation` | boolean | `true` = el renderer puede insertar guiones de separación silábica. Falso por defecto. |
| `active` | boolean | `false` = el locale existe en el sistema pero no está disponible para los usuarios. |
| `rtl` | boolean | Alias rápido de `direction == "rtl"`. Usado frecuentemente en templates. |
| `notes` | string | Notas de implementación para traductores y desarrolladores. No se muestra al usuario. |

---

## 3. Estructura Jerárquica de Traducciones

### 3.1 Modelo de namespace

Las traducciones se organizan en tres niveles:

```
{módulo}.{sección}.{clave}
```

- **módulo:** corresponde a un módulo compilable de Seam AMS (`core`, `assets`, `admin`, etc.)
- **sección:** agrupa semánticamente las claves dentro del módulo (formularios, listados, mensajes de error, etc.)
- **clave:** el texto específico

Este modelo reemplaza el espacio plano actual y permite:

- Cargar sólo las traducciones del módulo activo
- Administrar secciones independientemente desde la UI del módulo Admin
- Detectar automáticamente claves huérfanas o faltantes por módulo

### 3.2 Módulos estándar

| Módulo | Descripción |
|--------|-------------|
| `core` | Textos compartidos por todos los módulos: acciones comunes, mensajes de sistema, validaciones genéricas, navegación principal |
| `assets` | Módulo de gestión de activos: formularios de assets y sensores, maestros, eventos, reglas, alertas, FSM |
| `monitoring` | Módulo de monitoreo: dashboards, vistas en tiempo real, alertas activas |
| `reports` | Módulo de reportes: reportes históricos, exportaciones, analytics |
| `admin` | Módulo de administración: configuración del sistema, usuarios, locales, schemas, módulos |
| `field` | Namespace especial generado dinámicamente por el motor de formularios para los títulos de campos definidos en JSON Schema |

### 3.3 Secciones estándar por módulo

Cada módulo tiene secciones predefinidas. Los módulos pueden declarar secciones adicionales
propias.

#### Secciones del módulo `core`

| Sección | Contenido |
|---------|-----------|
| `app` | Nombre de la aplicación, tagline, versión |
| `nav` | Etiquetas de navegación principal y lateral |
| `actions` | Verbos de acción comunes: guardar, cancelar, eliminar, editar, crear, ver |
| `status` | Etiquetas de estado de assets: activo, silencioso, alerta, perdido |
| `errors` | Mensajes de error genéricos: campo requerido, formato inválido, valor fuera de rango |
| `confirm` | Textos de confirmación: `SeamConfirm.show()`, `SeamConfirm.showDanger()` |
| `empty` | Mensajes de estado vacío: "No hay registros", "Crea el primero..." |
| `pagination` | Textos de paginación: "Anterior", "Siguiente", "Mostrando N de M" |
| `datetime` | Nombres de meses, días de la semana, formatos relativos ("hace 5 minutos") |
| `validation` | Mensajes de error de validación de queryfy traducidos por campo y regla |

#### Secciones del módulo `assets`

| Sección | Contenido |
|---------|-----------|
| `list` | Encabezados de columnas, filtros, etiquetas del listado de assets |
| `forms` | Subsección contenedora de todos los formularios del módulo (ver §3.5) |
| `detail` | Etiquetas de la vista de detalle de asset |
| `types` | Textos del editor de tipos de asset |
| `sensors` | Textos del módulo de sensores |
| `events` | Textos del listado y detalle de eventos |
| `rules` | Textos del editor de reglas |
| `alerts` | Textos de alertas y notificaciones |
| `fsm` | Textos del editor de FSM |
| `masters` | Textos de las tablas maestras (regiones, marcas, custodios, etc.) |
| `masters.forms` | Subsección de formularios de maestros (ver §3.5) |
| `qr` | Textos de la vista de escaneo QR |
| `map` | Textos de la vista de mapa |

#### Secciones del módulo `admin`

| Sección | Contenido |
|---------|-----------|
| `users` | Gestión de usuarios, roles, permisos |
| `locales` | Editor de paquetes de locale, gestión de idiomas |
| `schemas` | Editor de schemas JSON del motor de formularios |
| `modules` | Gestión de módulos activos |
| `settings` | Configuración general del sistema |
| `settings.notifications` | Configuración de notificaciones |
| `settings.features` | Toggles de features |
| `settings.appearance` | Preferencias visuales |
| `forms` | Subsección contenedora de todos los formularios del módulo Admin (ver §3.5) |

### 3.4 Nomenclatura de formularios

Todos los formularios de un módulo se agrupan bajo la subsección `forms`, con una entrada
por formulario identificada por su nombre:

```
{módulo}.forms.{nombre_formulario}.{subsección}.{clave}
```

Esta estructura garantiza que cada formulario sea una unidad autocontenida y administrable
desde la UI del módulo Admin: al expandir `forms` en el editor de locales se ven todos los
formularios del módulo, y al entrar a cualquiera se ven sus subsecciones completas.

#### Subsecciones estándar de cada formulario

| Subsección | Contenido | Ejemplo de clave |
|------------|-----------|-----------------|
| `title.new` | Título en modo creación | `assets.forms.asset.title.new` → `"Nuevo Activo"` |
| `title.edit` | Título en modo edición | `assets.forms.asset.title.edit` → `"Editar Activo"` |
| `tab.{id}` | Label de cada tab | `assets.forms.asset.tab.principal` → `"Principal"` |
| `section.{id}` | Encabezado de sección | `assets.forms.asset.section.basico` → `"Básico"` |
| `field.{nombre}` | Label del campo | `assets.forms.asset.field.nombre` → `"Nombre"` |
| `field.{nombre}.help` | Texto de ayuda | `assets.forms.asset.field.id_padre.help` |
| `field.{nombre}.placeholder` | Placeholder | `assets.forms.asset.field.nombre.placeholder` |
| `errors.{campo}.{regla}` | Error específico del formulario | `assets.forms.asset.errors.tipo_id.required` |
| `actions.submit` | Botón guardar (si difiere del genérico) | `assets.forms.asset.actions.submit` → `"Crear Activo"` |
| `actions.cancel` | Botón cancelar (si difiere del genérico) | — |
| `actions.delete` | Botón eliminar (si difiere del genérico) | — |
| `detail.{nombre}.title` | Título de un panel de detalle embebido | `assets.forms.asset.detail.custodio_history.title` |
| `detail.{nombre}.col.{col}` | Encabezado de columna del panel | `assets.forms.asset.detail.custodio_history.col.fecha` |
| `detail.{nombre}.empty` | Estado vacío del panel | `assets.forms.asset.detail.custodio_history.empty` |
| `embed.{nombre}.title` | Título de un sub-formulario embebido | `assets.forms.asset.embed.sensor_binding.title` |
| `embed.{nombre}.field.{campo}` | Label de campo del sub-formulario | `assets.forms.asset.embed.sensor_binding.field.sensor_id` |
| `embed.{nombre}.field.{campo}.help` | Ayuda de campo del sub-formulario | — |
| `embed.{nombre}.actions.{acción}` | Acción del sub-formulario | `assets.forms.asset.embed.sensor_binding.actions.bind` |
| `embed.{nombre}.confirm.{acción}` | Confirmación de acción | `assets.forms.asset.embed.sensor_binding.confirm.unbind` |

#### Distinción entre `detail` y `embed`

**`detail`** — panel de sólo lectura embebido en el formulario. Muestra información
relacionada sin capacidad de edición. No genera datos al guardar el formulario principal.
Ejemplos: historial de cambios de custodio, eventos recientes del asset, estado actual del
FSM, lista de sensores vinculados (vista).

**`embed`** — sub-formulario embebido con capacidad de acción. Puede ejecutar operaciones
sobre entidades relacionadas sin salir del formulario principal. Puede abrirse en un modal
secundario o renderizarse inline. Tiene su propio endpoint y su propio ciclo de submit.
Ejemplos: vincular/desvincular sensor, asignar código QR, agregar una propiedad custom,
registrar un nuevo custodio desde el formulario de asset.

#### Formularios del módulo `assets`

| Nombre | Clave base | Descripción |
|--------|-----------|-------------|
| `asset` | `assets.forms.asset` | Formulario principal de asset (4 tabs) |
| `sensor` | `assets.forms.sensor` | Formulario de sensor |
| `asset_type` | `assets.forms.asset_type` | Editor de tipo de asset |
| `rule` | `assets.forms.rule` | Editor de regla |
| `fsm` | `assets.forms.fsm` | Editor de FSM |

#### Formularios del módulo `assets.masters`

| Nombre | Clave base | Descripción |
|--------|-----------|-------------|
| `region` | `assets.masters.forms.region` | Formulario de región |
| `moneda` | `assets.masters.forms.moneda` | Formulario de moneda |
| `marca` | `assets.masters.forms.marca` | Formulario de marca |
| `modelo` | `assets.masters.forms.modelo` | Formulario de modelo |
| `ubicacion` | `assets.masters.forms.ubicacion` | Formulario de ubicación |
| `custodio` | `assets.masters.forms.custodio` | Formulario de custodio |
| `centro_costo` | `assets.masters.forms.centro_costo` | Formulario de centro de costo |
| `alerta` | `assets.masters.forms.alerta` | Formulario de alerta |
| `prop_def` | `assets.masters.forms.prop_def` | Formulario de propiedad personalizada |
| `prop_assoc` | `assets.masters.forms.prop_assoc` | Formulario de asociación de propiedad |

#### Formularios del módulo `admin`

| Nombre | Clave base | Descripción |
|--------|-----------|-------------|
| `user` | `admin.forms.user` | Formulario de usuario |
| `user_password` | `admin.forms.user_password` | Cambio de contraseña |
| `locale` | `admin.forms.locale` | Editor de paquete de locale |
| `schema` | `admin.forms.schema` | Editor de schema JSON |
| `settings_general` | `admin.forms.settings_general` | Configuración general |
| `settings_notifications` | `admin.forms.settings_notifications` | Configuración de notificaciones |
| `settings_features` | `admin.forms.settings_features` | Toggles de features |
| `settings_appearance` | `admin.forms.settings_appearance` | Preferencias visuales |

#### Ejemplo de formulario completo con `detail` y `embed`

```json
"assets": {
  "forms": {
    "asset": {
      "title": {
        "new":  "Nuevo Activo",
        "edit": "Editar Activo"
      },
      "tab": {
        "principal": "Principal",
        "loc":       "Ubicación y Custodia",
        "ciclo":     "Ciclo de Vida",
        "finserv":   "Finanzas y Service"
      },
      "section": {
        "basico":          "Básico",
        "identificacion":  "Identificación",
        "caracterizacion": "Caracterización",
        "ubicacion":       "Ubicación Física",
        "custodia":        "Custodia",
        "fechas":          "Fechas de Ciclo de Vida",
        "finanzas":        "Finanzas",
        "service":         "Service e IoT"
      },
      "field": {
        "nombre":            "Nombre",
        "tipo_id":           "Tipo de Activo",
        "status":            "Estado",
        "criticidad":        "Criticidad",
        "connectivity_tier": "Conectividad",
        "marca_id":          "Marca",
        "modelo_id":         "Modelo",
        "custodio_id":       "Custodio",
        "id_padre":          "Asset Padre",
        "id_padre_help":     "Asset que contiene este activo en la jerarquía",
        "fecha_compra":      "Fecha Compra",
        "precio_compra":     "Precio Compra"
      },
      "errors": {
        "nombre":     { "required": "El nombre del activo es requerido" },
        "tipo_id":    { "required": "Debe seleccionar un tipo de activo" },
        "criticidad": { "required": "La criticidad es requerida" }
      },
      "actions": {
        "submit": "Guardar Cambios",
        "cancel": "Cancelar",
        "delete": "Eliminar Activo"
      },
      "detail": {
        "custodio_history": {
          "title": "Historial de Custodia",
          "col": {
            "fecha":    "Fecha",
            "custodio": "Custodio",
            "calidad":  "Calidad de Tenencia",
            "usuario":  "Registrado por"
          },
          "empty": "No hay cambios de custodia registrados"
        },
        "eventos": {
          "title": "Eventos Recientes",
          "col": {
            "fecha":   "Fecha",
            "tipo":    "Tipo",
            "valor":   "Valor",
            "sensor":  "Sensor"
          },
          "empty": "No hay eventos registrados",
          "link_all": "Ver todos los eventos →"
        },
        "fsm_state": {
          "title":    "Estado FSM",
          "estado":   "Estado actual",
          "no_fsm":   "Sin FSM asignado"
        },
        "sensores_vinculados": {
          "title": "Sensores Vinculados",
          "col": {
            "external_id": "ID Externo",
            "tipo":        "Tipo",
            "status":      "Estado",
            "last_seen":   "Última vez"
          },
          "empty": "No hay sensores vinculados"
        }
      },
      "embed": {
        "sensor_binding": {
          "title": "Vincular Sensor",
          "field": {
            "sensor_id":      "Sensor",
            "sensor_id_help": "Selecciona el sensor a vincular con este activo"
          },
          "actions": {
            "bind":   "Vincular",
            "unbind": "Desvincular"
          },
          "confirm": {
            "unbind": "¿Desvincular este sensor del activo? El sensor volverá a estar disponible."
          }
        },
        "qr_assign": {
          "title": "Asignar Código QR",
          "field": {
            "qr_code":      "Código QR",
            "qr_code_help": "Ingresa o escanea el código QR a asignar"
          },
          "actions": {
            "assign":   "Asignar",
            "unassign": "Desasignar"
          },
          "confirm": {
            "unassign": "¿Desasignar el código QR de este activo?"
          }
        }
      }
    }
  }
}

### 3.5 Formato del catálogo de traducciones

Dentro de cada módulo, las traducciones se organizan como un objeto anidado:

```json
{
  "modules": {
    "core": {
      "actions": {
        "save":   "Guardar",
        "cancel": "Cancelar",
        "delete": "Eliminar",
        "edit":   "Editar",
        "create": "Crear",
        "view":   "Ver",
        "back":   "Volver",
        "search": "Buscar",
        "filter": "Filtrar",
        "export": "Exportar",
        "import": "Importar"
      },
      "errors": {
        "required":          "Este campo es requerido",
        "invalid_email":     "Debe ser un email válido",
        "invalid_url":       "Debe ser una URL válida",
        "invalid_date":      "Debe ser una fecha válida",
        "min_length":        "Debe tener al menos {min} caracteres",
        "max_length":        "No puede superar {max} caracteres",
        "min_value":         "El valor mínimo es {min}",
        "max_value":         "El valor máximo es {max}",
        "invalid_pattern":   "El formato no es válido",
        "not_in_enum":       "Debe ser uno de los valores permitidos",
        "no_visible_tabs":   "No hay secciones disponibles para tu rol en este formulario",
        "no_visible_fields": "No hay campos disponibles para tu rol en esta sección"
      },
      "confirm": {
        "delete_title":   "Confirmar eliminación",
        "delete_message": "¿Eliminar {name}? Esta acción no se puede deshacer.",
        "generic_title":  "Confirmar acción",
        "ok":             "Confirmar",
        "cancel":         "Cancelar"
      },
      "pagination": {
        "previous":   "Anterior",
        "next":       "Siguiente",
        "showing":    "Mostrando {visible} de {total} {label}",
        "page":       "Página {current} de {total}"
      },
      "empty": {
        "no_records":  "No hay registros",
        "create_first": "Crea el primero para comenzar"
      }
    },
    "assets": {
      "list": {
        "title":            "Activos",
        "subtitle":         "{total} activo(s) registrado(s)",
        "col": {
          "name":    "Activo",
          "type":    "Tipo",
          "giai":    "GIAI",
          "status":  "Estado",
          "crit":    "Criticidad",
          "custodio":"Custodio",
          "actions": "Acciones"
        },
        "filter": {
          "all_status": "Todos los estados",
          "all_types":  "Todos los tipos"
        },
        "btn": { "new": "Nuevo Activo" }
      },
      "forms": {
        "asset": {
          "title": { "new": "Nuevo Activo", "edit": "Editar Activo" },
          "tab": {
            "principal": "Principal",
            "loc":       "Ubicación y Custodia",
            "ciclo":     "Ciclo de Vida",
            "finserv":   "Finanzas y Service"
          },
          "section": {
            "basico":          "Básico",
            "identificacion":  "Identificación",
            "caracterizacion": "Caracterización",
            "ubicacion":       "Ubicación Física",
            "custodia":        "Custodia",
            "fechas":          "Fechas de Ciclo de Vida",
            "finanzas":        "Finanzas",
            "service":         "Service e IoT"
          },
          "field": {
            "nombre":            "Nombre",
            "tipo_id":           "Tipo de Activo",
            "status":            "Estado",
            "criticidad":        "Criticidad",
            "connectivity_tier": "Conectividad",
            "marca_id":          "Marca",
            "modelo_id":         "Modelo",
            "custodio_id":       "Custodio",
            "id_padre":          "Asset Padre",
            "id_padre_help":     "Asset que contiene este activo en la jerarquía",
            "fecha_compra":      "Fecha Compra",
            "precio_compra":     "Precio Compra"
          },
          "errors": {
            "nombre":     { "required": "El nombre del activo es requerido" },
            "tipo_id":    { "required": "Debe seleccionar un tipo de activo" },
            "criticidad": { "required": "La criticidad es requerida" }
          },
          "actions": {
            "submit": "Guardar Cambios",
            "cancel": "Cancelar",
            "delete": "Eliminar Activo"
          },
          "detail": {
            "custodio_history": {
              "title": "Historial de Custodia",
              "col": {
                "fecha":    "Fecha",
                "custodio": "Custodio",
                "calidad":  "Calidad de Tenencia",
                "usuario":  "Registrado por"
              },
              "empty": "No hay cambios de custodia registrados"
            },
            "eventos": {
              "title":    "Eventos Recientes",
              "col": {
                "fecha":  "Fecha",
                "tipo":   "Tipo",
                "valor":  "Valor",
                "sensor": "Sensor"
              },
              "empty":    "No hay eventos registrados",
              "link_all": "Ver todos los eventos →"
            },
            "sensores_vinculados": {
              "title": "Sensores Vinculados",
              "col": {
                "external_id": "ID Externo",
                "tipo":        "Tipo",
                "status":      "Estado",
                "last_seen":   "Última vez"
              },
              "empty": "No hay sensores vinculados"
            }
          },
          "embed": {
            "sensor_binding": {
              "title": "Vincular Sensor",
              "field": {
                "sensor_id":      "Sensor",
                "sensor_id_help": "Selecciona el sensor a vincular con este activo"
              },
              "actions": {
                "bind":   "Vincular",
                "unbind": "Desvincular"
              },
              "confirm": {
                "unbind": "¿Desvincular este sensor? Volverá a estar disponible."
              }
            },
            "qr_assign": {
              "title": "Asignar Código QR",
              "field": {
                "qr_code":      "Código QR",
                "qr_code_help": "Ingresa o escanea el código QR a asignar"
              },
              "actions": {
                "assign":   "Asignar",
                "unassign": "Desasignar"
              },
              "confirm": {
                "unassign": "¿Desasignar el código QR de este activo?"
              }
            }
          }
        }
      }
    }
  }
}
```

---

## 4. Integración con el Motor de Formularios

### 4.1 Resolución de claves desde JSON Schema

> **Corrección, 2026-07-28:** esta sección usaba `{módulo}.form.field...` (singular
> "form", sin segmento de nombre de formulario) en las cadenas de resolución. La
> convención canónica de nomenclatura (§3.4) usa `{módulo}.forms.{nombre_formulario}.field...`
> (plural, con el nombre del formulario) -- confirmado contra el schema real de Asset,
> que ya usa `assets.forms.asset.tab.principal` para sus tabs/secciones desde FE-22.
> Corregido aquí para coincidir con §3.4, que es la convención autoritativa (tiene tabla
> completa, ejemplos reales, y ya está en producción). Encontrado auditando I18N_SPEC.md
> contra la implementación real al empezar T-13, no asumido.

El motor de formularios resuelve los textos de labels y ayuda a través del sistema i18n.
El campo `title` en el JSON Schema actúa como **clave de traducción por defecto**, no como
texto literal.

La resolución sigue este orden de prioridad:

```
1. {módulo}.forms.{nombre_formulario}.field.{nombre_campo}         — clave explícita en el formulario
2. {módulo}.forms.{nombre_formulario}.field.{nombre_campo}.label   — variante con sufijo .label
3. field.{nombre_campo}                                            — namespace global de campos
4. El valor literal de "title" en el schema                        — fallback: texto directo
```

Si ninguna clave resuelve, se usa el valor de `title` del schema tal cual.

#### Cómo se determinan `{módulo}` y `{nombre_formulario}`

Ninguno de los dos existía en ningún lado antes de T-13 -- ni como parámetro de
`Render()`, ni en `x-seam-form`. Se resuelven con dos niveles, el segundo opcional:

1. **Default del schema**: `x-seam-form.module` y `x-seam-form.name`. El caso común --
   un schema que siempre representa el mismo formulario (el Asset form siempre es
   `module: "assets"`, `name: "asset"`) lo declara una sola vez y todo caller lo hereda
   sin repetir nada.
2. **Override del caller**: `RenderContext.I18nModule` / `RenderContext.I18nForm`.
   Cuando cualquiera de los dos viene no-vacío, reemplaza el default del schema para
   *ese* eje específicamente (ambos son independientes, no una pareja obligatoria).
   Pensado para un schema reutilizable que se instancia varias veces -- una tarjeta de
   widget con su propio schema, embebida en distintos módulos, donde cada instancia
   necesita resolver bajo un namespace de traducción distinto sin que el schema mismo
   tenga que saber en qué módulo terminará viviendo cada vez.

Si ninguno de los dos niveles resuelve `{módulo}`/`{nombre_formulario}}` (schema sin
declarar y caller sin pasar override), la resolución de claves para ese formulario no
puede construirse -- cae directo al fallback de texto literal (paso 4 arriba).

### 4.2 Resolución de textos de ayuda

Para `x-seam-help`, la resolución sigue el mismo `{módulo}.forms.{nombre_formulario}.field...` que 4.1:

```
1. {módulo}.forms.{nombre_formulario}.field.{nombre_campo}.help    — clave en el formulario
2. field.{nombre_campo}.help                                       — namespace global
3. El valor literal de "x-seam-help"                                — fallback: texto directo
```

Esto permite que el schema defina un texto de ayuda en inglés como fallback, pero que el
paquete de locale lo sobreescriba con la traducción apropiada sin modificar el schema.

### 4.3 Namespace `field`

El namespace especial `field` en el módulo `core` contiene los labels canónicos de todos
los campos del sistema. Es el fallback global cuando un módulo no declara la traducción de
un campo:

```json
{
  "modules": {
    "core": {
      "field": {
        "nombre":           "Nombre",
        "descripcion":      "Descripción",
        "estado":           "Estado",
        "fecha_creacion":   "Fecha de Creación",
        "fecha_actualizacion": "Fecha de Actualización",
        "email":            "Email",
        "telefono":         "Teléfono",
        "direccion":        "Dirección"
      }
    }
  }
}
```

### 4.4 Mensajes de error de validación

Los mensajes de error de queryfy se traducen usando la sección `core.errors` con
interpolación de parámetros. El motor mapea el tipo de error de queryfy a la clave i18n:

| Error queryfy | Clave i18n | Parámetros |
|---------------|------------|------------|
| `field is required` | `core.errors.required` | — |
| `must be a valid email` | `core.errors.invalid_email` | — |
| `must be >= N` | `core.errors.min_value` | `{min}` |
| `must be <= N` | `core.errors.max_value` | `{max}` |
| `must be at least N chars` | `core.errors.min_length` | `{min}` |
| `must be at most N chars` | `core.errors.max_length` | `{max}` |
| `must match pattern` | `core.errors.invalid_pattern` | — |
| `must be one of: ...` | `core.errors.not_in_enum` | — |

### 4.5 Pluralización

Las claves que requieren pluralización incluyen el sufijo de forma plural según la regla
del locale. Para un locale con regla `one-other`:

```json
"pagination.showing": {
  "one":   "Mostrando {visible} de {total} {label}",
  "other": "Mostrando {visible} de {total} {label}"
}
```

Para árabe (6 formas):

```json
"pagination.showing": {
  "zero":  "لا يوجد {label}",
  "one":   "يُعرض {label} واحد من {total}",
  "two":   "يُعرض {label}ان من {total}",
  "few":   "يُعرض {visible} {label} من {total}",
  "many":  "يُعرض {visible} {label} من {total}",
  "other": "يُعرض {visible} {label} من {total}"
}
```

El renderer llama al pluralizador con el valor numérico y obtiene la forma correcta
antes de interpolar los parámetros.

---

## 5. Almacenamiento y Administración

### 5.1 Fuentes de paquetes

Los paquetes de locale tienen dos fuentes posibles:

**Embebidos** (`embed.FS`): Los locales base (`en`, `es`, `pt`) se compilan dentro del
binario como en la implementación actual. Son inmutables en tiempo de ejecución pero
sirven como fallback definitivo.

**Almacenados en xolu**: Los locales adicionales y las sobreescrituras de los locales base
se guardan en xolu bajo la entidad `locale_package`. Al iniciar, el servidor carga los
paquetes de xolu sobre los embebidos. Las ediciones desde el módulo Admin se persisten en
xolu y se recargan en caliente sin reinicio.

### 5.2 Recarga en caliente

El `Translator` expone un método `Reload(locale string) error` que recarga un paquete
específico desde xolu. El módulo Admin llama a este método después de guardar cambios en
el editor de traducciones, lo que hace que los textos actualizados aparezcan en la siguiente
request sin reiniciar el servidor.

### 5.3 Estructura en xolu

```
entidad: locale_package
id:      {code}          (ej: "es", "ar", "fr")
campos:
  code:    string
  data:    json           — el paquete completo serializado
  active:  boolean
  version: integer        — incrementado en cada edición
  updated_at: datetime
```

### 5.4 UI del módulo Admin — Editor de locales

El editor de paquetes de locale en el módulo Admin expone:

**Vista de locales:** listado de todos los paquetes instalados con su código, nombre,
dirección (RTL/LTR), estado (activo/inactivo), número de claves traducidas vs. pendientes,
y porcentaje de completitud respecto al locale base (`en`).

**Vista de módulo:** al seleccionar un locale y un módulo, se muestra el listado de
secciones del módulo con sus claves. Las claves faltantes (presentes en `en` pero ausentes
en el locale) se destacan como pendientes de traducción.

**Vista de sección:** al seleccionar una sección, se muestra un editor de dos columnas:
clave | texto en inglés (referencia) | texto en el locale (editable). Los campos editables
son textareas inline para permitir textos largos. El guardado es por sección entera.

**Vista de metadatos:** formulario para editar las propiedades de `meta` del paquete. Los
campos de `layout` y `typography` tienen preview visual en tiempo real.

**Importación/exportación:** el editor permite exportar un paquete como JSON para
traducciones offline, y reimportarlo una vez completado.

---

## 6. Implementación en Go

### 6.1 Estructura del paquete

```
internal/
  i18n/
    translator.go     — Translator refactorizado, carga jerárquica, pluralización
    locale.go         — LocalePackage, LocaleMeta, structs de metadatos
    pluralize.go      — motor de pluralización CLDR
    sources.go        — carga desde embed.FS y desde xolu
    middleware.go     — middleware HTTP que inyecta el Translator en el contexto
    locales/
      en.json         — paquete base embebido
      es.json         — paquete embebido
      pt.json         — paquete embebido
```

### 6.2 Interfaz pública

```go
// LocalePackage representa un paquete de locale completo.
type LocalePackage struct {
    Meta    LocaleMeta                            `json:"meta"`
    Modules map[string]map[string]interface{}    `json:"modules"`
}

// LocaleMeta contiene todos los metadatos del locale.
type LocaleMeta struct {
    Code       string          `json:"code"`
    Name       string          `json:"name"`
    NameEn     string          `json:"name_en"`
    Direction  string          `json:"direction"`  // "ltr" | "rtl"
    RTL        bool            `json:"rtl"`
    Script     string          `json:"script"`
    Typography TypographyMeta  `json:"typography"`
    Layout     LayoutMeta      `json:"layout"`
    Formats    FormatsMeta     `json:"formats"`
    Icons      IconsMeta       `json:"icons"`
    Input      InputMeta       `json:"input"`
    Plural     PluralMeta      `json:"pluralization"`
    Collation  string          `json:"collation"`
    Active     bool            `json:"active"`
    Notes      string          `json:"notes"`
}

// Translator provee resolución de claves con fallback jerárquico.
type Translator struct {
    packages map[string]*LocalePackage
    fallback string
    mu       sync.RWMutex
}

// T resuelve una clave jerárquica con interpolación de parámetros.
// clave formato: "modulo.seccion.clave" o "modulo.seccion.clave.forma_plural"
func (t *Translator) T(locale, key string, params ...map[string]any) string

// TP resuelve una clave con pluralización automática según el valor n.
func (t *Translator) TP(locale, key string, n int, params ...map[string]any) string

// Meta devuelve los metadatos del locale.
func (t *Translator) Meta(locale string) *LocaleMeta

// ForLocale retorna un helper bound a un locale específico.
func (t *Translator) ForLocale(locale string) *LocaleHelper

// Reload recarga un paquete desde xolu (hot reload).
func (t *Translator) Reload(locale string, pkg *LocalePackage) error

// LocaleHelper es un helper bound a un locale, para uso en handlers y templates.
type LocaleHelper struct {
    locale string
    t      *Translator
    Meta   *LocaleMeta
}

func (h *LocaleHelper) T(key string, params ...map[string]any) string
func (h *LocaleHelper) TP(key string, n int, params ...map[string]any) string
func (h *LocaleHelper) Dir() string       // "ltr" | "rtl"
func (h *LocaleHelper) RTL() bool
func (h *LocaleHelper) Icon(key string) string  // resuelve nombre de icono con overrides
func (h *LocaleHelper) DateFmt() string
func (h *LocaleHelper) NumberFmt(n float64) string
```

### 6.3 Interpolación de parámetros

La interpolación usa llaves simples como marcadores: `{nombre}`. El motor reemplaza
`{clave}` con el valor correspondiente del mapa de parámetros:

```go
// Uso:
t.T("es", "core.errors.min_length", map[string]any{"min": 3})
// → "Debe tener al menos 3 caracteres"

t.T("es", "core.confirm.delete_message", map[string]any{"name": "Contenedor A1"})
// → "¿Eliminar Contenedor A1? Esta acción no se puede deshacer."

t.TP("es", "core.pagination.showing", 42, map[string]any{
    "visible": 20,
    "total":   42,
    "label":   "activos",
})
// → "Mostrando 20 de 42 activos"
```

### 6.4 Middleware de locale

El middleware extrae el locale del usuario de la sesión y lo inyecta en el contexto HTTP:

```go
func LocaleMiddleware(t *Translator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            locale := localeFromSession(r)
            if !t.HasLocale(locale) {
                locale = t.Fallback()
            }
            ctx := context.WithValue(r.Context(), localeKey, t.ForLocale(locale))
            // Establece el header Content-Language
            w.Header().Set("Content-Language", locale)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// En los handlers:
helper := i18n.FromContext(r.Context())
helper.T("assets.list.title")  // → "Assets" / "Activos" / etc.
```

### 6.5 Integración con el renderer de layout RTL

Cuando el locale activo tiene `meta.rtl: true`, el renderer aplica:

```html
<!-- En el layout principal -->
<html lang="{locale}" dir="{direction}">
<head>
  <link rel="stylesheet" href="/static/css/app-rtl.css">
  <style>
    :root {
      --font-family: {typography.font_family};
      --font-size-adjust: {typography.font_size_adjust};
    }
  </style>
</head>
```

Los iconos direccionales (flechas de navegación, back/forward, chevrons) se voltean
automáticamente via clase CSS `icon-flip` cuando `layout.icon_flip: true`:

```css
/* app-rtl.css */
[dir="rtl"] .icon-flip {
  transform: scaleX(-1);
}
```

### 6.6 Resolución de iconos SVG con overrides

El helper `Icon(key)` resuelve el nombre de archivo del icono considerando los overrides
del locale:

```go
func (h *LocaleHelper) Icon(key string) string {
    if override, ok := h.Meta.Icons.Overrides[key]; ok {
        return "/static/icons/" + h.Meta.Icons.UseSet + "/" + override
    }
    // Fallback al conjunto base (Material Icons ligatures)
    return key  // nombre del ligature de Material Icons
}
```

---

## 7. Paquetes Base Incluidos

### 7.1 Locales embebidos en el MVP

| Código | Idioma | Dirección | Script | Estado |
|--------|--------|-----------|--------|--------|
| `en` | English | LTR | Latn | ✓ Base (fallback) |
| `es` | Español (Rioplatense) | LTR | Latn | ✓ Completo |
| `pt` | Português (Brasil) | LTR | Latn | ✓ Completo |

### 7.2 Locales planeados post-MVP (instalables desde Admin)

| Código | Idioma | Dirección | Script | Complejidad |
|--------|--------|-----------|--------|-------------|
| `fr` | Français | LTR | Latn | Baja |
| `de` | Deutsch | LTR | Latn | Baja |
| `it` | Italiano | LTR | Latn | Baja |
| `ar` | العربية | RTL | Arab | Alta — pluralización árabe, numerales |
| `he` | עברית | RTL | Hebr | Media — RTL, calendario hebreo opcional |
| `zh-CN` | 中文 (简体) | LTR | Hans | Media — sin pluralización, fuentes CJK |
| `zh-TW` | 中文 (繁體) | LTR | Hant | Media |
| `ja` | 日本語 | LTR | Jpan | Media — fuentes CJK, sin pluralización |
| `tr` | Türkçe | LTR | Latn | Baja — agrupación vocálica |
| `ru` | Русский | LTR | Cyrl | Media — pluralización eslava |
| `hi` | हिन्दी | LTR | Deva | Alta — fuentes Devanagari, numerales |

---

## 8. Consideraciones de Diseño

### 8.1 Compatibilidad con el sistema actual

El nuevo `Translator` mantiene compatibilidad con el método `T(locale, key, args...)` de
la implementación actual para facilitar la migración. Las claves planas existentes
(`"asset.title"`, `"nav.dashboard"`, etc.) se resuelven mapeándolas al namespace
jerárquico equivalente durante el período de migración.

La migración es progresiva: se convierte un módulo a la vez, comenzando por `core` y
siguiendo por `assets`.

### 8.2 Separación entre schema y traducción

El campo `title` en el JSON Schema del motor de formularios actúa como clave de traducción,
no como texto literal. Esto mantiene el schema neutral respecto al idioma — un mismo schema
sirve para todos los locales. Los textos viven exclusivamente en los paquetes de locale.

Esta separación tiene una consecuencia importante: al diseñar un nuevo schema de formulario,
el desarrollador escribe el `title` en inglés (que sirve como clave y como fallback), y el
paquete `es.json` contiene la traducción en su sección correspondiente.

### 8.3 Completitud de paquetes

El módulo Admin muestra el porcentaje de completitud de cada paquete comparando sus claves
con el paquete base `en`. Una clave ausente en un paquete no es un error — el `Translator`
hace fallback al locale base automáticamente. Las claves faltantes se muestran al usuario
en el idioma fallback, con una indicación visual sutil de que la traducción está pendiente
(opcional, configurable por tenant).

### 8.4 Textos definidos por el tenant

Los operadores DIY pueden necesitar personalizar textos de la aplicación con terminología
propia de su sector (ej: llamar a los "assets" "equipos" o "máquinas"). Esto se soporta
a través de un paquete de locale de tenant (`{locale}-{tenant_id}`) que sobreescribe sólo
las claves que el operador personaliza. El paquete del tenant tiene mayor prioridad que el
paquete base del locale.

Orden de resolución:

```
1. {locale}-{tenant_id}   — personalización del tenant
2. {locale}               — paquete estándar del locale
3. {fallback_locale}      — fallback (en)
4. La clave misma         — último recurso
```

---

## 9. Tabla de Propiedades de `meta` — Resumen

| Propiedad | Tipo | Requerida | Descripción |
|-----------|------|-----------|-------------|
| `code` | string | ✓ | BCP 47 |
| `name` | string | ✓ | Nombre en el idioma mismo |
| `name_en` | string | ✓ | Nombre en inglés |
| `name_native` | string | — | Escritura nativa (si difiere de `name`) |
| `region` | string | — | ISO 3166-1 alpha-2 |
| `direction` | string | ✓ | `"ltr"` \| `"rtl"` |
| `rtl` | boolean | ✓ | Alias de `direction == "rtl"` |
| `script` | string | — | ISO 15924 |
| `typography.font_family` | string | — | Familia tipográfica |
| `typography.font_size_adjust` | number | — | Factor de escala (default: 1.0) |
| `typography.line_height_adjust` | number | — | Factor altura de línea (default: 1.0) |
| `layout.icon_flip` | boolean | — | Voltear iconos direccionales en RTL |
| `layout.modal_placement` | string | — | `"left"` \| `"right"` |
| `layout.nav_side` | string | — | `"left"` \| `"right"` |
| `formats.date` | string | — | Patrón de fecha |
| `formats.datetime` | string | — | Patrón de fecha+hora |
| `formats.number_decimal` | string | — | Separador decimal |
| `formats.number_thousands` | string | — | Separador de miles |
| `formats.currency_position` | string | — | `"before"` \| `"after"` |
| `formats.first_day_of_week` | integer | — | 0=dom, 1=lun, 6=sáb |
| `icons.use_set` | string | — | Conjunto de iconos SVG |
| `icons.fallback_set` | string | — | Conjunto de fallback |
| `icons.overrides` | object | — | Sobreescrituras individuales de iconos |
| `input.numeral_system` | string | — | `"latn"` \| `"arab-indic"` \| ... |
| `input.keyboard_hint` | string | — | `inputmode` para móviles |
| `pluralization.rule` | string | ✓ | Regla CLDR |
| `pluralization.forms` | array | ✓ | Formas disponibles |
| `collation` | string | — | BCP 47 para ordenamiento |
| `hyphenation` | boolean | — | Habilitar separación silábica |
| `active` | boolean | ✓ | Disponible para usuarios |
| `notes` | string | — | Notas para traductores |

---

## 10. Riesgos Conocidos y Decisiones Pendientes

---

### 10.1 Deriva entre claves i18n y schemas de formulario

**Riesgo:** El motor de formularios resuelve labels de campos, secciones, tabs, details y
embeds como claves i18n. Si un schema cambia — se añade un campo, se renombra una sección,
se añade un embed nuevo — las claves i18n correspondientes pueden no existir en ningún
paquete de locale, lo que produce el fallback al valor literal de `title` en el schema.
Esto es correcto como comportamiento de fallback pero puede producir una UI mezclada donde
algunos textos están traducidos y otros aparecen en el idioma del schema (normalmente inglés).

Esta deriva es especialmente problemática para locales distintos de `en` cuando el sistema
está en producción activa con operadores DIY que modifican sus propios schemas: el operador
añade un campo, el label aparece en inglés para todos los usuarios, y no hay ninguna señal
visible de que la traducción falta.

**Mitigación recomendada:**

El módulo Admin debe exponer, en la vista de completitud de locale, una sección específica
de **claves derivadas de schemas** — claves que existen en los schemas almacenados en xolu
pero no en el paquete de locale. Estas claves se marcan como pendientes de traducción con
mayor prominencia que las claves de código faltantes, ya que tienen mayor probabilidad de
ser visibles al usuario en producción.

El endpoint que calcula la completitud de un locale debe consultar tanto los archivos de
código (claves estáticas) como los schemas en xolu (claves dinámicas) para generar el
porcentaje real de cobertura.

**Decisión pendiente:** definir si el cálculo de completitud de locale incluye claves de
schemas desde el MVP del editor de locales, o si se implementa en una fase posterior.

---

### 10.2 Claves de tenant personalizado y conflictos de namespace

**Riesgo:** Los paquetes de locale de tenant (`{locale}-{tenantID}`) permiten que un
operador sobreescriba cualquier clave del sistema. Si el operador sobreescribe una clave
del `core` (por ejemplo `core.actions.delete`) con un texto que cambia semánticamente la
acción (por ejemplo traduciéndola como "archivar" cuando la acción realmente elimina),
puede producir desorientación o errores operativos.

Adicionalmente, si el operador personaliza claves de otros módulos distintos al suyo propio
(`assets`, `admin`), una actualización del sistema que cambie el comportamiento asociado a
esa clave puede dejar la traducción del tenant desactualizada sin advertencia.

**Mitigación recomendada:**

Restringir los paquetes de tenant a sobreescrituras de claves dentro de un namespace
reservado: `tenant.*`. Las claves de `core.*`, `assets.*`, `admin.*` son de sólo lectura
para el operador. El operador puede definir sus propios términos (por ejemplo, llamar
"equipos" a los assets) creando alias en `tenant.terms.*` que el motor resuelve con
mayor prioridad que las claves estándar para ese tenant.

```json
"tenant": {
  "terms": {
    "asset":        "equipo",
    "asset_plural": "equipos",
    "sensor":       "dispositivo"
  }
}
```

El motor de formularios y la UI aplican `tenant.terms.*` cuando existen, sin que el
operador necesite sobreescribir individualmente cada clave que contiene la palabra "asset".

**Decisión pendiente:** definir el alcance de personalización permitido al tenant antes
de implementar el editor de locales en el módulo Admin.

## Referencia cruzada: módulo Developer

Updated: 2026-11-17

El módulo Developer (`docs/DEVELOPER_MODULE_CAMPAIGN.md`) implementa, para el rol developer, parte de lo que este documento plantea para la administración de textos: overrides por tenant, globales y por archivo sobre los locales embebidos (DEV-11 a DEV-13, DEC-4), recarga en caliente, historial de versiones, vista de completitud por idioma y lista de textos sin referencias en el código (DEV-18, DEV-19). Las claves de los formularios siguen la convención `<módulo>.forms.<formulario>.field.<campo>` que usa el motor de formularios. Lo que aquí se describe como editor de locales del módulo Admin debería construirse sobre ese código.

---

*Documento producido en sesión de diseño de arquitectura de Seam AMS, Mayo 2026.*  
*Copyright (c) 2026 haitch — h@ual.li*
