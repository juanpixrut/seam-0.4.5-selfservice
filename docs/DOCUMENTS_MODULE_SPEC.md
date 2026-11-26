# Seam AMS — Módulo de Gestión Documental
## Especificación de Diseño

**Versión:** 0.5  
**Fecha:** Mayo 2026  
**Estado:** Borrador interno  
**Repositorio:** github.com/ha1tch/seam (módulo `documents`)

---

## 1. Propósito y Alcance

El módulo de Gestión Documental es la capa de metadatos de negocio sobre los archivos
físicos que residen en el sistema de almacenamiento. Su responsabilidad es exclusivamente
dar significado a los archivos: qué tipo de documento es, a qué período corresponde, cuál
es su estado, y a qué entidades del sistema está vinculado.

### 1.1 Separación de responsabilidades

El módulo Documental no almacena archivos, no los mueve, no los renombra, y no produce
imágenes de previsualización. Estas responsabilidades pertenecen a otros componentes del
ecosistema Seam:

| Componente | Responsabilidad |
|------------|----------------|
| **seam-filer** | Almacenamiento físico de archivos. Implementa el contrato REST de fijuggler. Abstrae local FS y S3. |
| **fijuggler** | UI de navegación de archivos. Conoce rutas y bytes, no significado de negocio. |
| **thumble** | Generación de thumbnails y previsualizaciones. Acepta referencias a archivos y produce derivados WebP/JPEG/PNG. |
| **Módulo Documental** | Metadatos de negocio. Vincula referencias de fijuggler con entidades del sistema. |

El módulo Documental es deliberadamente fino: es una capa de integración con persistencia
en xolu, no una plataforma de gestión de contenidos.

### 1.2 Qué es un documento

Un documento es un registro en xolu que:

1. Tiene metadatos de negocio: tipo, descripción, fechas, estado, notas
2. Puede referenciar un archivo en seam-filer via `fijuggler_ref` (la ruta del archivo en
   el servidor de archivos)
3. Puede referenciar un registro de thumble via `thumble_asset_id` una vez que el archivo
   fue registrado para generación de previsualizaciones
4. Puede existir como **placeholder** — sin archivo adjunto — cuando se sabe que un
   documento debe existir pero aún no ha sido cargado

Un documento no es un archivo. Un archivo no es un documento. La vinculación entre
ambos es opcional en ambas direcciones: un archivo puede existir en seam-filer sin
ningún documento que lo describa, y un documento puede existir sin ningún archivo
asociado.

### 1.3 Qué es un adjunto

Un adjunto (`attachment`) es una referencia que vincula un documento con cualquier
entidad del sistema. Tiene cuatro componentes:

- El documento que se adjunta
- El tipo de entidad a la que se adjunta (`entity_type`): `"asset"`, `"sensor"`,
  `"custodio"`, `"region"`, o cualquier otra entidad registrada en xolu
- El ID de la entidad (`entity_id`)
- El contexto opcional dentro del formulario (`section_id`) — la sección del formulario
  donde fue creado el adjunto, como metadato de origen

Un mismo documento puede estar adjunto a múltiples entidades de cualquier tipo
simultáneamente. Por ejemplo, un manual técnico adjunto a un activo y al modelo de
equipo que lo describe, o un certificado de proveedor adjunto a un custodio y a todos
los activos bajo su custodia.

### 1.4 Posición en el mapa de módulos

```
core (formengine, i18n, listing, auth, xolu)
  ↓
masters (tipos de documento como tabla maestra)
  ↓
documents     ← este módulo
```

`documents` puede compilarse junto con cualquier módulo operacional sin importar el orden.
`assets`, `sensors`, y cualquier otro módulo que use adjuntos no son dependencias de
`documents` — la relación es inversa: los módulos operacionales consumen el tray del
módulo `documents` como componente htmx.

El módulo `documents` no depende de ningún módulo operacional específico. Los adjuntos
referencian entidades por tipo y ID genérico — no importan el módulo `assets` ni ningún
otro módulo de dominio. Los tipos de documento viven en `masters` como tabla configurable.

Esto permite que el módulo `documents` sea compilado e incluido en cualquier combinación
de módulos sin forzar dependencias transitivas.

---

## 2. Modelo de Datos

### 2.1 Entidad `document_type` (en masters)

Los tipos de documento son configurables por tenant. Viven en el módulo `masters` como
una tabla maestra más, con el mismo patrón de CRUD que regiones, marcas, y custodios.

```go
type DocumentType struct {
    ID          int64  `xolu:"id"`
    TenantID    string `xolu:"tenant_id"`
    Name        string `xolu:"name"`         // "Factura", "Manual", "Certificado", "Foto", etc.
    Description string `xolu:"description"`
    Icon        string `xolu:"icon"`         // Material Icons ligature
    Active      bool   `xolu:"active"`
}
```

Los tipos de documento base que se crean en el seed de cada tenant:

| Nombre | Icono |
|--------|-------|
| Factura | `receipt` |
| Manual | `menu_book` |
| Certificado | `verified` |
| Foto | `photo_camera` |
| Plano | `architecture` |
| Acta | `gavel` |
| Contrato | `handshake` |
| Garantía | `security` |
| Informe | `summarize` |
| Otro | `attach_file` |

### 2.2 Entidad `document`

```go
type Document struct {
    ID             int64      `xolu:"id"`
    TenantID       string     `xolu:"tenant_id"`

    // Metadatos de negocio
    TypeID         int64      `xolu:"type_id"`         // FK → document_type
    Title          string     `xolu:"title"`           // Título descriptivo — aparece en listados y pills
    Description    string     `xolu:"description"`     // Resumen corto en texto plano — una línea
    IssueDate      *time.Time `xolu:"issue_date"`      // Fecha de emisión del documento
    ExpiryDate     *time.Time `xolu:"expiry_date"`     // Fecha de vencimiento (si aplica)
    IssuedBy       string     `xolu:"issued_by"`       // Emisor (organismo, proveedor, persona)
    ReferenceCode  string     `xolu:"reference_code"`  // Número de factura, código de certificado, etc.
    Status         string     `xolu:"status"`          // "present" | "pending" | "expired"
    Notes          string     `xolu:"notes"`           // Notas editoriales en Markdown. Widget: x-seam-input:"markdown".
                                                      // El renderer sanitiza el HTML antes de mostrar.
    Tags           []string   `xolu:"tags"`            // Etiquetas de libre elección. Autocomplete desde tags usados en el tenant.

    // Referencias a servicios externos
    FijugglerRef   string     `xolu:"fijuggler_ref"`   // Ruta en seam-filer. Vacío si placeholder.
    ThumbleAssetID string     `xolu:"thumble_asset_id"` // ID en thumble. Vacío hasta que se registre.
    ThumbURL       string     `xolu:"thumb_url"`        // URL del thumbnail generado. Vacío hasta job completado.
    MimeType       string     `xolu:"mime_type"`        // MIME detectado por seam-filer al vincular.
    FileSizeBytes  int64      `xolu:"file_size_bytes"`  // Tamaño en bytes del archivo vinculado.
    // Nota: el texto extraído del archivo (full_text) NO vive en este struct.
    // Vive en la entidad file_index de seam-indexer, keyed por fijuggler_ref.
    // La búsqueda compuesta une document (tags, title, description) con file_index (full_text).

    // Auditoría
    CreatedBy      string     `xolu:"created_by"`
    UpdatedBy      string     `xolu:"updated_by"`
    CreatedAt      time.Time  `xolu:"created_at"`
    UpdatedAt      time.Time  `xolu:"updated_at"`
}
```

**Estados del campo `Status`:**

| Valor | Significado |
|-------|-------------|
| `present` | Documento existe y tiene archivo adjunto |
| `pending` | Placeholder — se sabe que debe existir pero no hay archivo aún |
| `expired` | `ExpiryDate` está en el pasado. Calculado, no ingresado manualmente. |

El estado `expired` se calcula al leer: si `expiry_date` no es nil y es anterior a la
fecha actual, el documento se presenta como expirado independientemente del valor
almacenado en `status`.

### 2.3 Entidad `attachment`

```go
type Attachment struct {
    ID          int64     `xolu:"id"`
    TenantID    string    `xolu:"tenant_id"`
    DocumentID  int64     `xolu:"document_id"`  // FK → document
    EntityType  string    `xolu:"entity_type"`  // "asset" | "sensor" | "custodio" | "region" | ...
    EntityID    int64     `xolu:"entity_id"`    // ID de la entidad en su módulo
    SectionID   string    `xolu:"section_id"`   // ID de sección del form (contexto de origen). Puede estar vacío.
    LinkedBy    string    `xolu:"linked_by"`    // usuario que creó el adjunto
    LinkedAt    time.Time `xolu:"linked_at"`
}
```

El campo `section_id` es contexto de origen, no un filtro obligatorio. La consulta de
adjuntos de una entidad devuelve todos los adjuntos de esa entidad independientemente de
`section_id`. El campo existe para que el tray del formulario pueda destacar los documentos
adjuntados desde su propia sección, sin excluir los demás.

### 2.4 Grafo xolu

Los adjuntos se representan también como aristas en el grafo de xolu entre la entidad
`document` y la entidad de destino, con el label de relación `"adjunto_a"`. Esto permite
consultas Sulpher para encontrar todos los documentos de cualquier entidad y todas las
entidades que comparten un documento:

```
// Todos los documentos de un asset
MATCH (d:document)-[:adjunto_a]->(a:asset) WHERE a._id = "asset:42" RETURN d

// Todos los documentos de un sensor
MATCH (d:document)-[:adjunto_a]->(s:sensor) WHERE s._id = "sensor:7" RETURN d

// Todas las entidades que comparten un documento
MATCH (d:document {_id:"document:15"})-[:adjunto_a]->(e) RETURN e

// Documentos adjuntos a un custodio
MATCH (d:document)-[:adjunto_a]->(c:custodio) WHERE c._id = "custodio:3" RETURN d
```

---

## 3. Flujo de Vinculación de Archivos

El proceso de vincular un archivo a un documento sigue estos pasos, todos orquestados
por el módulo Documental sin intervención del usuario:

```
1. Usuario navega en fijuggler y selecciona un archivo (o lo selecciona desde el
   file picker del tray del formulario)

2. El módulo recibe: fijuggler_ref (ruta del archivo), document_id

3. Módulo consulta seam-filer: GET /api/files/get?path={fijuggler_ref}
   → obtiene: nombre, mime_type, size_bytes

4. Módulo registra el archivo en thumble: POST /assets
   {
     "mode": "local",    (si seam-filer habla local FS)
     "path": "{ruta_física}"
   }
   o
   {
     "mode": "s3",
     "bucket": "{bucket}",
     "key": "{key}"
   }
   → obtiene: thumble_asset_id

5. Módulo crea jobs de renderizado eager en thumble:

   **Para documentos de texto/oficina (PDF, DOCX, XLSX, etc.):**
   - `POST /assets/{thumble_asset_id}/jobs` por cada página, perfil `thumb`, fmt `jpeg`,
     calidad 75. El número de páginas se obtiene del response de registro de thumble
     (campo `page_count` que thumble debe exponer — ver §FILE_HANDLING_SPEC).
   - Jobs enviados en batch; thumble procesa en paralelo según su pool de workers.

   **Para imágenes:**
   - Un solo job de thumbnail (256px square, WebP) para el pill del tray.
   - Un job de preview (ancho 1200px, WebP) para el visor.

   **Para audio:**
   - Un job de forma de onda (`waveform`, 600×120px, WebP/PNG).
   - Un job de clip preview (30 segundos, mono OGG 64kbps, fade in/out) — nuevo tipo
     de job en thumble (ver §FILE_HANDLING_SPEC).

   **Para video:**
   - N jobs de frame JPEG según el algoritmo de muestreo adaptativo (ver §5.5 y
     §FILE_HANDLING_SPEC). El número de frames y sus timestamps se calculan a partir
     de la duración del video, que thumble expone tras el registro.

6. Módulo actualiza el documento en xolu:
   - fijuggler_ref = ruta del archivo
   - thumble_asset_id = ID de thumble
   - mime_type = mime detectado
   - file_size_bytes = tamaño
   - status = "present"
   - preview_type = tipo inferido del mime ("document" | "image" | "audio" | "video" | "none")
   - page_count = número de páginas (para documentos de texto/oficina)
   - audio_duration = duración en segundos (para audio)
   - video_frames = array de VideoFrame con index, timestamp, thumb_url (para video,
     poblado a medida que los jobs de thumble completan)

7. En background (goroutine o job queue): sondeo de jobs de thumble hasta completar,
   actualizando `thumb_url` (thumbnail principal para el tray pill) y `video_frames`
   en el documento xolu a medida que cada job termina.
```

Si thumble no está disponible, los pasos 4-5 se omiten y `thumble_asset_id` queda vacío.
El módulo funciona en modo degradado: los documentos se muestran con icono de tipo MIME
en lugar de thumbnail. El campo `thumb_url` puede completarse en cualquier momento
posterior si thumble vuelve a estar disponible.

---

## 4. Tray de Adjuntos en Formularios

### 4.1 Integración con el motor de formularios

Las secciones del formulario que tienen `attachable: true` en su definición muestran un
tray de adjuntos al pie de la sección. El tray es generado por el motor de formularios
a partir del `RenderContext` — no por el módulo Documental directamente.

Al renderizar una sección attachable en modo edición (`IsEdit: true`), el motor:

1. Consulta vía OQL los adjuntos de la entidad:
   ```sql
   SELECT * FROM attachments
   WHERE entity_type = '{entity_type}' AND entity_id = {entity_id}
   ORDER BY linked_at DESC
   ```
   donde `entity_type` y `entity_id` provienen del `RenderContext` — el motor ya
   conoce qué entidad está siendo editada.

2. Para cada adjunto, resuelve el documento referenciado y su `thumb_url`

3. Renderiza el tray con los primeros N documentos (N configurable, default 8)

### 4.2 Tres estados del tray

El tray sigue el mismo modelo de tres estados que fijuggler usa en quag:

**Cerrado** — la sección no tiene documentos adjuntos. El tray no se muestra o muestra
únicamente una zona de drop discreta sin contenido.

**Pills** — la sección tiene documentos. Se muestran hasta 8 pills inline al pie de la
sección. Cada pill contiene:
- Thumbnail WebP si `thumb_url` está disponible, icono de tipo MIME como fallback
- Título del documento truncado a ~20 caracteres
- Badge de estado si es `pending` o `expired`

**Modal** — si hay más de 8 documentos, un enlace "Ver todos ({n})..." abre un modal
con la interfaz completa del módulo Documental, filtrada por la entidad y la sección.
También al hacer click en "Adjuntar documento" desde el tray.

### 4.3 Acciones disponibles desde el tray

| Acción | Descripción |
|--------|-------------|
| Click en pill | Abre el visor del documento (§5.5) en un modal mediano con el preview correspondiente al tipo de archivo |
| "Adjuntar" | Abre el modal del gestor documental para seleccionar o crear un documento |
| "Ver todos" | Abre el modal del gestor documental filtrado por esta entidad |
| Hover sobre pill | Tooltip con título completo, tipo, fecha |

Desde el tray nunca se accede directamente a fijuggler. La navegación de archivos ocurre
dentro del modal del gestor documental.

### 4.4 Propiedad `attachable` en secciones

En el JSON Schema de un formulario, la propiedad `attachable` se declara en la definición
de una sección dentro de `x-seam-form`:

```json
{
  "id":         "custodia",
  "label":      "assets.forms.asset.section.custodia",
  "layout":     "2-col",
  "fields":     ["custodio_id", "calidad_tenencia", "vto_comodato"],
  "attachable": true
}
```

Las secciones sin `attachable: true` no muestran tray. El Form Designer en el módulo
Admin expone esta propiedad como un toggle en el configurador de sección.

---

## 5. Interfaz del Módulo Documental

El módulo Documental tiene dos superficies de UI:

1. **El tray embebido** en formularios de cualquier entidad (§4)
2. **El gestor documental completo** — accesible como modal desde el tray, o directamente
   desde la navegación del módulo

### 5.1 Vista principal del gestor

El gestor documental muestra una lista de documentos con filtros:

| Columna | Contenido |
|---------|-----------|
| Thumbnail | Imagen WebP de thumble, icono MIME como fallback |
| Tipo | Badge con nombre del tipo de documento |
| Título | Título descriptivo |
| Estado | `present` / `pending` / `expired` con color |
| Entidades | Número de entidades a las que está adjunto (con desglose por tipo si hay más de uno) |
| Fecha | Fecha de emisión si existe, fecha de creación si no |
| Acciones | Ver, Editar, Desvincular, Eliminar |

**Filtros disponibles:**
- Tipo de documento
- Estado (present / pending / expired)
- Etiqueta (filtra documentos que contienen la etiqueta seleccionada)
- Entidad específica (cuando se abre desde un tray)
- Rango de fechas de emisión
- Búsqueda de texto libre (ver §5.6 — búsqueda compuesta con scoring)

### 5.2 Vista de detalle de documento

Al abrir un documento, se muestra:

**Panel izquierdo:** visor del archivo — ver §5.5 para la descripción completa del
comportamiento por tipo de archivo. El visor ocupa todo el panel izquierdo y adapta su
presentación al tipo MIME del archivo vinculado.

**Panel derecho:** metadatos del documento en modo editable o de sólo lectura según
permisos:
- Tipo de documento (select desde document_type)
- Título
- Resumen (texto plano, una línea)
- Etiquetas (input de tags con autocomplete desde tags usados en el tenant)
- Código de referencia
- Fecha de emisión
- Fecha de vencimiento
- Emisor
- Notas (Markdown — widget `x-seam-input: "markdown"`, renderizado sanitizado)
- Archivo vinculado (ruta en seam-filer con botón "Abrir en gestor de archivos")
- Estado calculado
- Creado por / Fecha de creación
- Última modificación por / Fecha de modificación

**Panel inferior:** lista de entidades a las que está adjunto este documento, agrupadas
por tipo (`assets`, `sensors`, `custodios`, etc.), con botón para desvincular de cada una.

### 5.3 Creación de documento

Formulario simple con los campos de §2.2. El campo "Archivo" muestra el selector de
seam-filer (fijuggler en modo pill/expanded) para vincular un archivo existente.

La creación de documento no implica upload de archivo. El archivo debe existir previamente
en seam-filer. El módulo documental no hace upload — la UI dirige al usuario a fijuggler
si necesita subir un archivo primero.

### 5.4 Panel "Abrir en gestor de archivos"

En la vista de detalle, un botón abre fijuggler en un modal fullscreen (SeamModal) con
el directorio del archivo pre-seleccionado. Esto es el punto de acceso a fijuggler desde
el módulo Documental.

### 5.5 Visor de documentos

El visor es el componente central de la vista de detalle y del modal de preview que
se abre al hacer click en un pill del tray. Se comporta distinto según el tipo de
archivo. Todos los derivados de visualización son producidos por thumble en el momento
de vinculación del archivo (eager rendering) o bajo demanda si no están disponibles.

#### Documentos de texto y oficina

**Aplica a:** PDF, DOCX, ODT, XLSX, ODS, PPTX y variantes, MD, TXT

Todas las páginas son pre-renderizadas como JPEGs de calidad media (calidad 75) por
thumble en el momento de vinculación del archivo. Un documento de N páginas produce
N entradas en thumble — cada página es un derivative independiente con su propio
`page` parameter.

El visor muestra:
- La página actual como imagen JPEG centrada en el panel
- Una tira de miniaturas a la izquierda o debajo (configurable) que muestra todas las
  páginas como thumbnails pequeños en miniatura, scrollable
- Navegación de página: botones anterior/siguiente, campo numérico para saltar a página
  específica, indicador "Página N de M"
- Zoom básico: ajustar al ancho, ajustar a la altura, 100%

Al hacer click en una miniatura de la tira, la página correspondiente se carga en el
panel principal. La carga es instantánea si el JPEG ya está en el cache de thumble,
o dispara un job de thumble bajo demanda si por alguna razón no está disponible.

El número total de páginas (`page_count`) se almacena en el documento xolu al momento
de vinculación (proveniente de thumble tras el eager rendering), de modo que el visor
puede construir la tira de navegación sin round-trips adicionales.

#### Imágenes

**Aplica a:** JPEG, PNG, GIF, TIFF, BMP, WebP, SVG, HEIC y otros formatos de imagen

Las imágenes se muestran en una galería simple. Si el documento tiene un solo archivo,
se muestra centrada con opciones de zoom (ajustar, 100%, libre). Si el documento tiene
múltiples archivos (no es el caso en el modelo actual, ya que un documento = un archivo)
se muestran como galería con thumbnails.

El visor usa directamente la URL de thumble (`/assets/{id}/preview`) para la imagen a
tamaño de visor, y `thumb_url` (256px square) para la miniatura en el tray. No se
sirve el archivo original desde seam-filer para la preview — sólo para la descarga
(permiso `Download`).

#### Audio

**Aplica a:** MP3, OGG, FLAC, WAV, AAC y otros formatos de audio

thumble produce dos derivados para archivos de audio:

1. **Forma de onda** — imagen WebP/PNG de la forma de onda completa del audio, usada
   como representación visual del archivo en el panel principal del visor y en el pill
   del tray
2. **Preview de audio** — clip OGG Vorbis mono de 30 segundos, los primeros 30 segundos
   del archivo original, con fade-in de 0.5 segundos al inicio y fade-out de 2 segundos
   al final. Bitrate 64kbps. Tamaño aproximado: ~240KB

El visor de audio muestra:
- La imagen de forma de onda a pantalla completa en el panel
- Un reproductor de audio minimalista posicionado sobre la forma de onda: botón
  play/pause, barra de progreso, indicador de tiempo (00:00 / 00:30)
- El clip de preview se reproduce desde seam-filer proxy via el endpoint de contenido
- Nota de duración: "Vista previa: 30 segundos" si el archivo es más largo
- Para usuarios con permiso `Download`: botón de descarga del archivo original

El clip de preview se produce con ffmpeg dentro de thumble. No requiere ninguna nueva
dependencia externa.

#### Video

**Aplica a:** MP4, MKV, AVI, MOV, WebM y otros formatos de video

thumble produce un conjunto de frames JPEG distribuidos a lo largo de la duración del
video, siguiendo el algoritmo de muestreo adaptativo:

| Duración | Intervalo entre frames | Frames generados |
|----------|----------------------|-----------------|
| < 10 min | 30 segundos | hasta 20 |
| 10–30 min | 1 minuto | 10–30 |
| 30–60 min | 2 minutos | 15–30 |
| 60–150 min | 5 minutos | 12–30 |
| > 150 min | 5 minutos, máximo 30 | 30 |

El objetivo es siempre aproximarse a 20–30 frames independientemente de la duración,
con un intervalo mínimo de 30 segundos para clips cortos y un máximo absoluto de 30
frames para cualquier duración.

El visor de video muestra:
- El frame seleccionado actualmente (frame 1 por defecto) a pantalla completa
- Una tira horizontal de frames en miniatura debajo del panel principal, scrollable
  horizontalmente, con el timestamp de cada frame visible (00:00, 01:00, 02:00, etc.)
- Click en cualquier miniatura → ese frame se muestra a pantalla completa
- Navegación con teclado: ← y → para frame anterior/siguiente
- Indicador de posición: "Frame N de M — 00:00:00"
- El número total de frames y sus timestamps se almacenan en el documento xolu al
  momento de vinculación, de modo que la tira se construye sin round-trips adicionales

Para usuarios con permiso `Download`: botón de descarga del archivo original.

#### Formatos sin soporte

Para tipos de archivo que thumble no puede previsualizar (ejecutables, archivos
comprimidos, formatos propietarios desconocidos), el visor muestra:
- Icono de tipo MIME grande (Material Icons)
- Nombre del archivo
- Tamaño y tipo MIME
- Para usuarios con permiso `Download`: botón de descarga

#### Visor desde el tray (modal pequeño)

Cuando el usuario hace click en un pill del tray (permiso `Preview` o superior), se
abre un modal de tamaño mediano (`width: "xl"`) con el visor. El comportamiento es
idéntico al visor en la vista de detalle pero sin el panel derecho de metadatos ni el
panel inferior de entidades. El modal tiene un enlace "Ver detalle completo" que navega
a la vista de detalle del documento.

#### Campos adicionales en `document` para soporte del visor

El modelo `document` en §2.2 se extiende con los siguientes campos para soportar el
visor sin round-trips adicionales a thumble:

```go
// Campos adicionales para el visor — poblados por el flujo de vinculación
PageCount      int            `xolu:"page_count"`       // Para documentos de texto/oficina. 0 si no aplica.
VideoFrames    []VideoFrame   `xolu:"video_frames"`     // Para video. Vacío si no aplica.
AudioDuration  int            `xolu:"audio_duration"`   // Duración en segundos. 0 si no aplica.
HasAudioClip   bool           `xolu:"has_audio_clip"`   // true si thumble generó el clip OGG preview.
PreviewType    string         `xolu:"preview_type"`     // "document" | "image" | "audio" | "video" | "none"
```

```go
// VideoFrame describe un frame extraído de un video.
type VideoFrame struct {
    Index     int    `xolu:"index"`      // 1-based
    Timestamp int    `xolu:"timestamp"`  // segundos desde el inicio
    ThumbURL  string `xolu:"thumb_url"`  // URL del derivative en thumble
}
```

El campo `PreviewType` permite al visor saber inmediatamente qué componente renderizar
sin inferirlo del MIME type en cada carga.

---

## 6. Rutas del Módulo

### 6.1 Rutas de UI

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/documents` | `DocumentsList` | todos los roles |
| GET | `/documents/new` | `DocumentNew` | operator, manager, admin |
| POST | `/documents` | `DocumentCreate` | operator, manager, admin |
| GET | `/documents/{id}` | `DocumentDetail` | todos los roles |
| GET | `/documents/{id}/edit` | `DocumentEdit` | operator, manager, admin |
| POST | `/documents/{id}` | `DocumentUpdate` | operator, manager, admin |
| POST | `/documents/{id}/delete` | `DocumentDelete` | manager, admin |
| GET | `/documents/{entity_type}/{entity_id}` | `DocumentsByEntity` | todos los roles |

### 6.2 Rutas de API (htmx y tray)

| Método | Ruta | Handler | Descripción |
|--------|------|---------|-------------|
| GET | `/api/v1/documents` | `APIDocumentsList` | Listado con filtros OQL |
| GET | `/api/v1/documents/{id}` | `APIDocumentGet` | Detalle con thumb_url resuelto |
| GET | `/api/v1/documents/{id}/viewer` | `APIDocumentViewer` | HTML del visor para modal htmx |
| GET | `/api/v1/documents/{id}/page/{n}` | `APIDocumentPage` | Redirect a thumble para página N (documentos de texto/oficina) |
| GET | `/api/v1/documents/{id}/frame/{n}` | `APIDocumentFrame` | Redirect a thumble para frame N (video) |
| POST | `/api/v1/documents/{id}/link` | `APIDocumentLink` | Vincula archivo de seam-filer al documento |
| POST | `/api/v1/documents/{id}/unlink-file` | `APIDocumentUnlinkFile` | Desvincula archivo (documento pasa a pending) |
| GET | `/api/v1/attachments/{entity_type}/{entity_id}` | `APIAttachmentsList` | Adjuntos de una entidad |
| GET | `/api/v1/attachments/{entity_type}/{entity_id}/section/{section_id}` | `APIAttachmentsBySection` | Adjuntos de una entidad por sección |
| POST | `/api/v1/attachments/{entity_type}/{entity_id}` | `APIAttachmentCreate` | Adjunta documento a entidad |
| DELETE | `/api/v1/attachments/{entity_type}/{entity_id}/{attachment_id}` | `APIAttachmentDelete` | Desvincula documento de entidad |
| GET | `/api/v1/documents/tray/{entity_type}/{entity_id}/{section_id}` | `APITrayContent` | HTML del tray para htmx |

### 6.3 Ruta del gestor de archivos (delegación a fijuggler)

| Método | Ruta | Handler | Descripción |
|--------|------|---------|-------------|
| GET | `/files` | `FilesPage` | Página que monta `<fj-app>` con REST provider apuntando a seam-filer |
| GET | `/api/files/*` | Proxy → seam-filer | Proxy transparente al servidor de archivos |

La ruta `/files` sirve la página completa de fijuggler. El proxy `/api/files/*` reenvía
las llamadas del REST provider de fijuggler al servicio seam-filer, permitiendo que
fijuggler opere sin CORS y sin que el browser conozca la URL de seam-filer.

---

## 7. i18n del Módulo

El módulo sigue la nomenclatura de `docs/I18N_SPEC.md`. Las claves viven bajo
`documents.forms.*` y `documents.list.*`.

### 7.1 Claves del listado

```json
"documents": {
  "list": {
    "title":           "Documentos",
    "col.thumbnail":   "Vista previa",
    "col.type":        "Tipo",
    "col.title":       "Título",
    "col.status":      "Estado",
    "col.assets":      "Activos",
    "col.date":        "Fecha",
    "col.actions":     "Acciones",
    "filter.all_types": "Todos los tipos",
    "filter.all_status": "Todos los estados",
    "btn.new":         "Nuevo Documento",
    "status.present":  "Presente",
    "status.pending":  "Pendiente",
    "status.expired":  "Vencido"
  }
}
```

### 7.2 Claves del tray

```json
"documents": {
  "tray": {
    "empty":        "Sin documentos adjuntos",
    "attach":       "Adjuntar documento",
    "view_all":     "Ver todos ({n})...",
    "drop_hint":    "Arrastra documentos aquí",
    "pending_badge":"Pendiente",
    "expired_badge":"Vencido"
  }
}
```

### 7.3 Formulario de documento

```json
"documents": {
  "forms": {
    "document": {
      "title": { "new": "Nuevo Documento", "edit": "Editar Documento" },
      "section": {
        "basic":    "Información básica",
        "file":     "Archivo",
        "dates":    "Fechas",
        "entities": "Entidades vinculadas"
      },
      "field": {
        "type_id":        "Tipo de documento",
        "title":          "Título",
        "description":    "Resumen",
        "tags":           "Etiquetas",
        "notes":          "Notas",
        "reference_code": "Código de referencia",
        "issue_date":     "Fecha de emisión",
        "expiry_date":    "Fecha de vencimiento",
        "issued_by":      "Emisor",
        "notes":          "Notas",
        "fijuggler_ref":  "Archivo vinculado",
        "fijuggler_ref_help": "Selecciona un archivo del gestor de archivos"
      },
      "actions": {
        "submit":       "Guardar Documento",
        "open_files":   "Abrir gestor de archivos",
        "unlink_file":  "Desvincular archivo"
      }
    },
    "document_type": {
      "title": { "new": "Nuevo Tipo de Documento", "edit": "Editar Tipo" },
      "field": {
        "name":        "Nombre",
        "description": "Descripción",
        "icon":        "Ícono"
      }
    }
  }
}
```

---

## 8. Estructura del Módulo

```
modules/
  documents/
    module.go           — registro del módulo, rutas, middleware
    handlers/
      documents.go      — CRUD de documentos
      attachments.go    — gestión de adjuntos
      tray.go           — HTML del tray para htmx
      files.go          — proxy hacia seam-filer y página de fijuggler
      link.go           — flujo de vinculación con seam-filer y thumble
    pages/
      documents.go      — páginas UI del módulo documental
    locales/
      documents.en.json
      documents.es.json
```

---

## 9. Dependencias de Servicios

### 9.1 seam-filer (requerido)

El módulo necesita seam-filer para:
- Obtener metadatos de archivos al vincular (`mime_type`, `file_size_bytes`)
- Servir archivos al usuario a través del proxy `/api/files/*`
- Montar la UI de fijuggler apuntada a seam-filer

Si seam-filer no está disponible, el módulo opera en modo degradado:
- Los documentos existentes se muestran con su metadata almacenada en xolu
- La vinculación de archivos nuevos no está disponible
- El gestor de archivos no puede abrirse
- Los thumbnails existentes (`thumb_url` ya almacenados) siguen funcionando

### 9.2 thumble (opcional)

El módulo usa thumble para:
- Registrar archivos recién vinculados
- Solicitar generación de thumbnails
- Obtener URLs de thumbnails para mostrar en pills y detalle

Si thumble no está disponible:
- Los documentos se muestran con icono de tipo MIME en lugar de thumbnail
- El campo `thumble_asset_id` permanece vacío
- Los thumbnails ya generados y almacenados en `thumb_url` siguen funcionando
- El módulo intenta registrar con thumble en cada vinculación y falla silenciosamente si
  no está disponible, registrando la condición en el log

### 9.3 Degraded mode summary

| seam-filer | thumble | Estado del módulo |
|-------------|---------|-------------------|
| ✓ | ✓ | Completo |
| ✓ | ✗ | Sin thumbnails; iconos MIME como fallback |
| ✗ | ✓ | Sin gestión de archivos; metadata existente visible |
| ✗ | ✗ | Sólo metadata de documentos existentes; sin nuevas vinculaciones |

---

### 5.6 Búsqueda compuesta de documentos

La búsqueda de documentos combina cuatro señales con un score ponderado. El handler
de búsqueda ejecuta las cuatro consultas y combina los resultados en memoria antes
de devolver la lista ordenada por score descendente.

#### Señales y pesos

| Señal | Peso | Fuente | Consulta |
|-------|------|--------|---------|
| Tag match | 40 | xolu `document.tags` | Coincidencia exacta de tag = 40; prefijo = 20 |
| Title match | 30 | xolu `document.title` | Empieza con = 30; contiene = 15 |
| Description match | 20 | xolu `document.description` | Contiene = 20 |
| FTS match | 10 | xolu FTS5 `file_index.full_text` | Score FTS5 normalizado a 0–10 |

**Score total máximo: 100.**

Un documento sin archivo vinculado (placeholder) no tiene `full_text` y por lo tanto
no contribuye señal FTS — su score máximo es 90. Esto es correcto: los placeholders
son menos relevantes en búsqueda de contenido.

#### Flujo del handler de búsqueda

```
1. Recibir query string q
2. OQL: SELECT id, title, description, tags FROM documents
        WHERE tenant_id = ? AND (title LIKE '%q%' OR description LIKE '%q%'
        OR tags CONTAINS q)  -- resultado A
3. OQL FTS: SELECT fijuggler_ref, snippet, rank FROM file_index
            WHERE full_text MATCH q AND tenant_id = ?  -- resultado B
4. Para cada documento en A: calcular tag_score + title_score + desc_score
5. Para cada documento en A que tenga fijuggler_ref en B: añadir fts_score
6. Ordenar por score descendente
7. Devolver primeros 50 resultados con snippet de FTS si disponible
```

#### Tag autocomplete

El endpoint `GET /api/v1/documents/tags?q={prefix}` devuelve los tags más usados en
el tenant que empiezan con el prefijo dado:

```sql
SELECT TOP 10 tag, COUNT(*) as n
FROM document_tags   -- vista o tabla expandida de tags
WHERE tenant_id = ? AND tag LIKE '{prefix}%'
GROUP BY tag
ORDER BY n DESC
```

Los pesos son configurables por tenant en una versión posterior. Para el MVP están
hardcodeados en el handler.

---

## 10. Riesgos Conocidos y Decisiones Pendientes

### 10.1 Consistencia entre seam-filer y xolu

**Riesgo:** Si un archivo es eliminado de seam-filer directamente (sin pasar por el
módulo Documental), los documentos que lo referencian vía `fijuggler_ref` quedan con
una referencia rota. El módulo no tiene mecanismo de notificación desde seam-filer.

**Mitigación recomendada:** El módulo verifica la existencia del archivo al abrir la
vista de detalle de un documento. Si seam-filer retorna 404 para el `fijuggler_ref`,
el documento se marca visualmente como "archivo no encontrado" y `status` se actualiza
a `pending`. Esta verificación es lazy (sólo al abrir el detalle) para no impactar
el rendimiento del listado.

**Decisión pendiente:** Definir si seam-filer debe publicar eventos de eliminación de
archivos (via NATS) que el módulo Documental consuma para actualizar `fijuggler_ref` a
vacío automáticamente. Recomendado cuando NATS esté integrado en el stack completo.

### 10.2 Consistencia del thumbnail

**Riesgo:** El `thumb_url` almacenado en xolu puede quedar desactualizado si thumble
evicta el derivative del cache (por expiración o por invalidación de firma al cambiar
el archivo fuente). La URL sigue existiendo en xolu pero retorna 404 en thumble.

**Mitigación recomendada:** El módulo maneja 404 de thumble en el renderizado del tray
y la vista de detalle: si `thumb_url` retorna 404, fallback a icono MIME y se re-encola
un job de thumble en background. Una vez completado el job, se actualiza `thumb_url`
en xolu.

**Decisión pendiente:** Definir tiempo de vida del `thumb_url` almacenado en xolu.
Opción A: sin TTL, manejar 404 en caliente. Opción B: TTL explícito en el documento
con campo `thumb_expires_at`, regenerar proactivamente. Opción A es suficiente para
el MVP.

### 10.3 Carga del tray en formularios complejos

**Riesgo:** Un formulario con muchas secciones attachable (ej: 4 secciones, cada una
con adjuntos) dispara múltiples consultas OQL al renderizar, más las resoluciones de
`thumb_url` de cada documento. Puede impactar el tiempo de carga del formulario.

**Mitigación recomendada:** El tray se carga lazy via htmx (`hx-trigger="revealed"`)
al igual que los paneles `detail`. El formulario renderiza sin el contenido del tray;
el contenido llega en una segunda request cuando el tab/sección se hace visible. Esto
está alineado con el patrón ya especificado en `docs/FORM_ENGINE_SPEC.md` para paneles
`detail`.

### 10.4 Un documento adjunto a muchas entidades

**Riesgo:** Un documento adjunto a cientos de entidades (manual de equipo de una flota
grande, o un certificado de proveedor vinculado a todos sus activos) puede hacer que la
vista de detalle del documento tarde en cargar la lista de entidades vinculadas.

**Mitigación recomendada:** La lista de entidades vinculadas en la vista de detalle se
pagina con los primeras 20, agrupadas por tipo, con un "Ver todos" que abre una vista
dedicada. La consulta usa el índice compuesto de `attachment.document_id` en xolu.

---

*Documento producido en sesión de diseño de arquitectura de Seam AMS, Mayo 2026.*  
*Copyright (c) 2026 haitch — h@ual.li*
