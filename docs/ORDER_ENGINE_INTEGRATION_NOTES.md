# Seam AMS — Notas de Integración: Motor de Órdenes (CMMS)
## Respuesta técnica a la especificación (v1.0)

Versión: 0.1
Fecha: 2026-07-28
Estado: Borrador interno — respuesta de arquitectura, no reemplaza la especificación original
Documento de origen: "AMS Orders -- anotada.docx" (v1.0)

---

## 1. Propósito

Este documento evalúa "Motor de Órdenes Configurables" v1.0 contra la arquitectura
actual y planificada de Seam AMS (form engine, xolu, módulos Assets Management /
Documents / Settings / Administration), con dos objetivos:

1. Identificar qué partes del Motor de Órdenes pueden construirse reutilizando
   infraestructura existente o en desarrollo, en lugar de reimplementarse desde cero.
2. Documentar los tipos de campo y otras discrepancias entre lo que el Motor de
   Órdenes necesita y lo que el motor de formularios dinámico
   (`FORM_ENGINE_SPEC.md` v0.3) actualmente ofrece.

No es una crítica al diseño original — el diseño del Motor de Órdenes es sólido y,
de hecho, gran parte de su estructura (Header configurable, Workflow, versionado)
sigue patrones muy similares a los que Seam ya adoptó para Assets. Este documento
traza los puentes concretos.

---

## 2. Resumen ejecutivo

La mayor parte del Motor de Órdenes no necesita construirse desde cero: mapea, con
distinto grado de fidelidad, sobre primitivas que Seam ya tiene o está construyendo.

| Componente del Motor de Órdenes | Se apoya en | Estado |
|---|---|---|
| Header (campos configurables) | JSON Schema + extensiones `x-seam-*` (FORM_ENGINE_SPEC.md) | Construible ahora, con gaps de tipos de dato (§4) |
| Field Definition | Propiedades JSON Schema + `x-seam-input`/`x-seam-source`/etc. | Mismo motor que Assets — no debe reimplementarse |
| Workflow (estados y transiciones) | xolu `/fsm` (`pkg/client/machine.go`) | Directo — es exactamente lo que `/fsm` hace |
| Numeración | xolu `/gen/seq` (secuencias con `increment_by`, `cycle`, `min_val`/`max_val`) | Directo para secuencia simple; reinicio anual/mensual necesita verificarse (§3.3) |
| Acciones Automáticas | Definiciones de evento de xolu atadas a transiciones de FSM (`ListEventDefs`/`EventDef`), no `internal/webhook` a nivel de aplicación | Recomendado: usar el mecanismo nativo de xolu — atómico con el commit (§3.4) |
| Business Rules | Mismo patrón que `internal/rules` (Condition→Action), pero scope distinto | Nuevo motor, mismo patrón de diseño — no es reutilización directa (§3.5) |
| Scope (selección de activos) | Integración directa con Assets Management | Nuevo — no existe hoy un mecanismo de "restricción de selección" en `x-seam-form` (§4) |
| Blocks | Nuevo concepto — sin equivalente actual | Genuinamente nuevo (§3.6) |

Lo genuinamente nuevo que el Motor de Órdenes introduce: el patrón **Blocks**
(sub-unidades con ciclo de vida y responsable propios dentro de una misma orden),
el concepto **Scope** como restricción declarativa de selección de activos, y dos
entidades de dominio que hoy no existen en Seam: **Cuadrilla** y **Empresa/Proveedor**
(§5).

---

## 3. Mapeo detallado

### 3.1 Tipo de Orden → sin cambios necesarios

`OrderType` tal como está descrito (catálogo simple: código, nombre, ícono, color,
activo) no necesita ningún mecanismo nuevo — es estructuralmente idéntico a
cualquier tabla maestra simple que ya existe en Settings/masters (ej. Marca,
Región). Puede construirse con el mismo patrón.

### 3.2 Order Model → Header, Blocks, Lines

- **Header**: es, campo por campo, lo mismo que un formulario de Asset ya hace con
  `x-seam-form` a nivel raíz — tabs opcionales, secciones, campos. No hay
  necesidad de un mecanismo nuevo aquí.
- **Blocks + Lines**: esto es genuinamente nuevo. El widget `repeatable` de
  FORM_ENGINE_SPEC.md (sub-formulario con filas dinámicas) es estructuralmente
  parecido pero no equivalente: `repeatable` no tiene noción de responsable
  asignado, estado propio, ni fechas de inicio/fin por fila. Blocks necesita su
  propio nivel de extensión — probablemente `x-seam-blocks` como hermano de
  `x-seam-form`, no una reutilización de `repeatable`. Ver §6 para la pregunta
  abierta.

### 3.3 Numeración → xolu `/gen/seq`, con una brecha por verificar

El `Sequence` que devuelve xolu (`pkg/client/types_schema.go`) tiene `Start`,
`Current`, `IncrementBy`, `Cycle`, `MinVal`, `MaxVal` — ningún campo de reinicio
calendario (anual/mensual). "Reinicio anual" / "Reinicio mensual" en la spec
es un reinicio atado a fecha, no un wraparound numérico — son conceptos
distintos. Hay que verificar si xolu lo soporta en alguna otra forma antes de
asumir que `/gen/seq` cubre esto completo; si no, la lógica de reinicio
calendario tendría que vivir en la capa de Seam/Order Engine, generando una nueva
secuencia (o reseteando `Current`) al cruzar el límite de período.

### 3.4 Acciones Automáticas → recomendación: usar el sistema nativo de xolu, no `internal/webhook`

**Confirmado 2026-07-28** (ver `docs/RESOLVED.md`, T-12): xolu `/fsm` es el motor
de FSM en tiempo de ejecución para todo Seam, sin ambigüedad — tanto Assets
Management como el Workflow de este módulo se construyen sobre él. Ya no es
sólo una recomendación de este documento.

xolu ya implementa exactamente el patrón que describe la spec: una
definición de evento dice "cuando esta transición de FSM confirma, entregar este
payload a este webhook" (README de xolu). Esto se ejecuta atómicamente con el
commit — si Seam-app está caído, el evento igual se dispara, porque vive en la
capa de datos, no en la capa de aplicación.

`internal/webhook` (que ya existe en Seam) es un mecanismo de entrega de
webhooks a nivel de aplicación — funciona, pero depende de que el proceso de
Seam esté corriendo y de que un handler se ejecute en el momento correcto. Para
"Aprobar Orden → Ejecutar un Webhook" la opción correcta es casi con certeza
registrar una definición de evento en xolu atada a la transición del Workflow,
no replicar esa lógica en Seam.

Las acciones que actualizan datos del propio activo (ubicación, custodio,
responsable) también encajan naturalmente como parte del mismo mecanismo —
quedan del lado del compromiso atómico, no como una segunda llamada separada
después del commit.

### 3.5 Business Rules → mismo patrón que `internal/rules`, motor distinto

`internal/rules` ya implementa Condition→Action con soporte para umbrales,
duración, tasa de cambio, geofence, cambio de estado, y condición de FSM — pero
está scoped específicamente a alertas de assets (`AssetTypeIDs`, `AssetTags`).
No es directamente reutilizable para Business Rules de órdenes (que operan sobre
campos de Header/Blocks y transiciones de Workflow, no sobre telemetría de
assets), pero es el patrón de diseño a seguir en lugar de inventar uno nuevo.
Considerar si vale la pena generalizar `internal/rules` en lugar de bifurcar dos
motores de reglas paralelos — pregunta abierta, no una respuesta obvia (§6).

### 3.6 Blocks → genuinamente nuevo

Nada en Seam hoy tiene el concepto de "misma entidad con múltiples sub-unidades,
cada una con su propio estado de FSM y su propio responsable." La forma más
directa de construirlo sobre xolu: cada Block es su propia entidad (con su
propia definición de FSM), vinculada a la Order vía un campo REF (arista de
grafo `orden → block`), que es exactamente cómo xolu ya modela relaciones. No
hace falta ningún primitivo nuevo de xolu para esto — es composición de lo que
ya existe (entidades + REF + FSM por entidad), no una brecha.

---

## 4. Brecha de tipos de campo

Comparación entre "Tipos de Datos Soportados" (spec, sección de Field
Definition) y la tabla de widgets de FORM_ENGINE_SPEC.md v0.3 §14.

| Tipo (Motor de Órdenes) | Widget FORM_ENGINE_SPEC equivalente | Estado |
|---|---|---|
| Boolean | `checkbox` / `toggle` | ✓ existe |
| Integer, Decimal | `number` | ⚠ parcial — ningún widget expone precisión/escala decimal; xolu sí tiene `format: "decimal"` a nivel de schema (docs/JSON_SCHEMA.md de xolu) pero FORM_ENGINE_SPEC no lo expone como `x-seam-input` |
| Text, LongText | `text`, `textarea` | ✓ existe |
| Date, Time, DateTime | `date`, `time`, `datetime-local` | ✓ existe |
| DomainSelect | `select` (enum o FK) | ✓ existe |
| DomainMultiSelect | — | ✗ falta — no hay ningún widget de selección múltiple en la tabla de widgets |
| Document | `document-links` | ⚠ parcial — `document-links` es una lista de enlaces nombre+url+tipo, no un campo de documento único; además debería integrar con el módulo Documents (T-04), no ser un widget aislado |
| Image | — | ✗ falta — sin widget de captura/preview de imagen (relevante para fotos de antes/después en órdenes de mantenimiento) |
| Signature | — | ✗ falta — sin widget de firma. Necesidad común en CMMS (conformidad de técnico/supervisor) |
| Color | `color` | ✓ existe |
| URL | `url` | ✓ existe |
| Email | `email` | ✓ existe |
| Phone | `tel` | ✓ existe (mismo concepto, nombre distinto) |
| GPS | — | ✗ falta como widget dedicado — el schema de ejemplo de Asset ya tiene `latitude`/`longitude` como campos `number` sueltos, no como un tipo GPS unificado (captura desde dispositivo / selector en mapa) |
| QR Code | — | ✗ falta como tipo de campo — pero Seam ya tiene `internal/qrcode` (generación + checksum) para el tagging de assets. Pregunta abierta: ¿este tipo de campo es para *generar* un QR nuevo, o para *escanear* uno existente durante la ejecución de la orden? Son casos de uso distintos (§6) |
| EAN, UPC | — | ✗ falta — código de barras lineal, simbología distinta de QR; no hay nada parecido hoy en Seam |
| Usuario | `select` (FK) + `x-seam-source` | ✓ existe conceptualmente, aunque no hay un widget `user-picker` explícito en la tabla |
| Cuadrilla | — | ✗ falta la *entidad*, no sólo el widget — §5 |
| Empresa | — | ✗ falta la *entidad*, no sólo el widget — §5 |
| Activo | `asset-type-select` | ✓ existe |
| Ubicación | `select` (FK) | ✓ existe (mismo patrón que `ubicacion_fisica_id` en el schema de ejemplo de Asset) |
| Custodio | `select` (FK) + `x-seam-fk-popover` | ✓ existe (mismo patrón que `custodio_id`) |

Además de los tipos de dato, Field Definition en la spec incluye varios
atributos que no tienen extensión `x-seam-*` equivalente hoy:

- **Unidad de medida** — sin extensión. Relevante no sólo para Órdenes sino para
  Calibración/Inspección en general (valores con unidad: kg, psi, °C).
- **Máscara de ingreso** — sin extensión (`x-seam-input-mask` no existe).
- **Validación personalizada** — FORM_ENGINE_SPEC cubre validación estándar de
  JSON Schema vía queryfy (pattern, min/max, required) pero no un punto de
  extensión para lógica de validación arbitraria.
- **Código externo (ERP)** — sin extensión definida; probablemente una propiedad
  JSON Schema simple sin necesidad de lógica especial, pero no está documentada
  en FORM_ENGINE_SPEC.md.
- **Orden de visualización** — no es una brecha: ya está cubierto implícitamente
  por la posición del campo dentro del array `fields` de cada `section` en
  `x-seam-form`. El equipo de CMMS no necesita un campo numérico separado para
  esto.

---

## 5. Entidades de dominio nuevas

Verificado contra los masters actuales (`internal/ui/masters_pages.go`):
Regiones, Monedas, Centros de Costo, Marcas, Custodios, Alertas (master),
PropDefs, Modelos, Ubicaciones, PropAssocs. Dos conceptos que la spec
necesita no existen en ningún lado de Seam hoy:

- **Cuadrilla** (conjunto de usuarios) — usada como responsable de un Block. No
  es lo mismo que un rol de Administración; es un agrupamiento operativo de
  usuarios, ortogonal a los roles.
- **Empresa / Proveedor** — usada tanto como responsable de un Block
  (tercerización) como en el ejemplo de campos de Header ("Proveedor"). Nótese
  que Centro de Costo sí existe ya como master (`CentrosCosto`), así que no
  todos los campos de ejemplo del Header están sin cubrir — sólo
  Empresa/Proveedor.

Ninguna de las dos tiene un dueño obvio todavía: ¿viven en Administration (junto
a usuarios/roles), en Settings, o en un nuevo espacio compartido en `core`?
Cuadrilla en particular tiene sentido cerca de Administration porque es
fundamentalmente un agrupamiento de usuarios.

---

## 6. Preguntas abiertas para el equipo de CMMS

1. **Blocks**: ¿se modela como una nueva extensión `x-seam-blocks`, o como una
   entidad separada con su propio schema y una relación REF hacia la Order? Esto
   determina si Blocks vive dentro del form engine o al lado de él.
2. **QR Code como tipo de campo**: ¿generar o escanear? Si es escanear, la
   interacción es fundamentalmente distinta (requiere cámara del dispositivo) de
   cualquier widget existente.
3. **Business Rules**: ¿generalizar `internal/rules` para que sirva tanto a
   alertas de assets como a reglas de orden, o mantener dos motores separados con
   el mismo patrón de diseño? Ninguna opción es obviamente correcta — la primera
   evita duplicación pero acopla dos dominios que hoy son independientes.
4. **Reinicio calendario de numeración**: confirmar si xolu soporta esto en algún
   lugar no revisado todavía, antes de asumir que hay que construirlo en la capa
   de Seam.
5. **Dónde viven Cuadrilla y Empresa/Proveedor** — Administration, Settings, o
   `core`.

---

## 7. Próximos pasos sugeridos (no iniciados)

- Ampliar FORM_ENGINE_SPEC.md v0.3 con los widgets/extensiones faltantes de §4
  antes de que el equipo de CMMS empiece a depender del motor — o documentar
  explícitamente cuáles quedan fuera del alcance del primer corte y por qué.
- Definir el quinto módulo (nombre de trabajo: "Orders" o "Maintenance") con sus
  dependencias: Assets Management (Scope), Documents (Document/Image/Signature
  deberían pasar por ahí, no por un widget aislado), Settings (Field
  Definitions), Administration (Usuario, Cuadrilla).
- Verificar el soporte de xolu para reinicio calendario de secuencias antes de
  diseñar Numbering.

---

*Documento producido en respuesta a "AMS Orders -- anotada.docx" (v1.0),
2026-07-28.*
*Copyright (c) 2026 haitch — h@ual.li*
