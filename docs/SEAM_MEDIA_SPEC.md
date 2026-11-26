# Seam Media — Especificación del Subsistema de Archivos
## Especificación de Diseño

**Versión:** 0.3  
**Fecha:** Mayo 2026  
**Estado:** Borrador interno  
**Repositorio destino:** github.com/ha1tch/seam-media (pendiente de creación)  
**Ubicación provisional:** seam-app/docs/SEAM_MEDIA_SPEC.md

---

## 1. Propósito y Alcance

Seam Media es el subsistema responsable del almacenamiento, navegación y previsualización
de archivos en Seam AMS. Está compuesto por tres componentes independientes que se
integran a través de contratos REST bien definidos:

| Componente | Rol | Repositorio |
|-----------|-----|-------------|
| **fijuggler** | UI de navegación de archivos (Lit web component) | github.com/ha1tch/fijuggler |
| **seam-filer** | Servidor de archivos en Go — implementa el contrato REST de fijuggler, abstrae local FS y S3, coordina con thumble y xolu | github.com/ha1tch/seam-filer (nuevo) |
| **thumble** | Servicio de generación de thumbnails y previsualizaciones | github.com/ha1tch/thumble |
| **seam-indexer** | Extracción de texto y cola de procesamiento para LLM futuro | github.com/ha1tch/seam-indexer (nuevo) |

Los tres componentes pueden desplegarse y desarrollarse de forma independiente. Seam App
los consume a través de la interfaz `media.Client` (§9), lo que permite desarrollo en
paralelo sin bloqueos.

### 1.1 Lo que Seam Media no es

Seam Media no conoce el concepto de "documento", "activo", "adjunto" o ninguna entidad
de negocio de Seam AMS. Seam Media es un subsistema de infraestructura: sabe sobre
archivos, bytes, rutas y derivados visuales. El significado de negocio vive exclusivamente
en el módulo Documental de Seam App.

### 1.2 Flujo completo de extremo a extremo

```
1.  Usuario abre /files en Seam App
2.  Seam App sirve la página con <fj-app> apuntando al proxy /api/files/*
3.  Usuario navega a Inbox/ (directorio de aterrizaje por defecto)
4.  Usuario arrastra un archivo → fijuggler emite upload via REST provider
5.  Seam App proxy → seam-filer POST /api/files/upload
6.  seam-filer escribe bytes en storage (local FS o S3), bajo /{tenant_id}/{path}
7.  seam-filer crea file_record en xolu (ThumbState/IndexState/SummaryState = "pending")
8.  seam-filer registra el archivo en thumble: POST /assets {mode, path/key} — background
9.  thumble responde con asset_id, page_count/duration → seam-filer actualiza file_record
10. seam-filer encola eager rendering jobs en thumble si aplica (§3.5.1) — background
11. seam-filer encola index_job en xolu para seam-indexer (§12.5) — background
12. seam-filer responde a fijuggler con listing estándar — inmediato
13. [background] thumble procesa jobs → seam-filer actualiza file_record.ThumbState = "done"
14. [background] seam-indexer extrae texto → escribe file_index, actualiza file_record.IndexState = "done"
15. Usuario crea documento en módulo Documental, vincula fijuggler_ref
16. Módulo Documental consulta file_record en xolu → obtiene mime, size, thumble_asset_id, ThumbState
17. Módulo Documental almacena page_count / video_frames / thumb_url en document entity xolu
18. Búsqueda de documentos: score compuesto une document (tags/title/desc) con file_index (full_text)
19. Formulario de activo carga tray → renderiza pills con thumb_url desde document entity
20. Usuario hace click en pill → visor abre → page N via /api/v1/documents/{id}/page/{n}
    → proxy a thumble GET /assets/{thumble_asset_id}/preview?page={n}
```

---

## 2. fijuggler — Componente de UI

### 2.1 Descripción

fijuggler v0.2.0 es un web component Lit que implementa un gestor de archivos completo:
árbol de directorios, vista grilla/lista, drag-and-drop, búsqueda, panel de preview,
menú de contexto (abrir, nueva carpeta, cortar, copiar, pegar, renombrar, eliminar,
seleccionar todo). Toda la lógica de almacenamiento se delega a un proveedor REST
pluggable — seam-filer implementa ese contrato.

### 2.2 Modo de despliegue en Seam App

fijuggler se monta como componente fullscreen en la ruta `/files`:

```html
<!DOCTYPE html>
<html style="width:100%;height:100%;overflow:hidden">
<body style="width:100%;height:100%;margin:0">
  <fj-app
    rest-url="/api/files"
    theme="tailwind-dark"
  ></fj-app>
  <script type="module" src="/static/vendor/fijuggler.min.js"></script>
</body>
</html>
```

El atributo `rest-url="/api/files"` apunta al proxy de Seam App (§8.1), que reenvía
las llamadas a seam-filer. fijuggler nunca conoce la URL de seam-filer.

### 2.3 Vendoring

fijuggler se vendoriza siguiendo el patrón existente en `web/static/vendor/`:

```
web/static/vendor/
  fijuggler.min.js        — bundle ESM compilado con esbuild
  fj-tailwind-dark.css    — tema Tailwind dark (usado por Seam App)
  fj-tailwind-light.css   — tema Tailwind light
```

Entrada en `VENDOR.md`:

```
## fijuggler v0.2.0
Source:  github.com/ha1tch/fijuggler/releases/tag/v0.2.0
SHA-256: [calculado al vendorizar]
Re-vendor: cd fijuggler && npm ci && npm run build:esm
           cp dist/fijuggler.min.js web/static/vendor/
           cp dist/themes/fj-tailwind-dark.css web/static/vendor/
```

### 2.4 Panel de tres estados en el tray de formularios

En el contexto del tray de adjuntos (§4 de DOCUMENTS_MODULE_SPEC), fijuggler no se
embebe directamente. El tray usa el patrón de tres estados de quag:

| Estado | Descripción | Disparador |
|--------|-------------|-----------|
| **Cerrado** | Sin tray visible | No hay adjuntos o permiso `hidden` |
| **Pills** | Lista compacta de documentos adjuntos (§4 DOCUMENTS_MODULE_SPEC) | Sección con adjuntos |
| **Modal** | fijuggler fullscreen en SeamModal | "Abrir gestor de archivos" |

La apertura del modal de fijuggler se implementa via `SeamModal.open({width: "fullscreen", url: "/files"})`. No se embebe `<fj-app>` en el DOM de la página principal — se carga en un iframe o en una ruta dedicada según la implementación del modal.

### 2.5 Internacionalización

fijuggler v0.2.0 tiene aproximadamente 30 strings hardcodeados en inglés. Para
internacionalización completa se requieren:

1. **Reemplazo de strings** — agregar una propiedad `strings` al componente `<fj-app>`
   que recibe un objeto plano `{key: translated_string}`. Los strings del componente
   leen de este objeto con fallback al inglés. Trabajo estimado: medio día en fijuggler.

2. **Locale para fechas** — los tres call sites de `toLocaleDateString('en-GB')`
   necesitan usar el locale pasado por la propiedad. Un cambio de una línea por call site.

3. **Reemplazo de `prompt()` y `confirm()`** — los diálogos nativos del browser
   (nueva carpeta, confirmación de eliminación) deben reemplazarse por componentes
   propios para ser traducibles. Trabajo estimado: medio día.

**Para el MVP:** fijuggler opera en inglés. Se documenta como deuda técnica conocida.
El resto de Seam App funciona en el locale del tenant.

---

## 3. seam-filer — Servidor de Archivos

### 3.1 Propósito

seam-filer es un servicio Go que implementa el contrato REST de diez endpoints que
fijuggler espera de su proveedor REST (§3.3), más endpoints adicionales para integración
con Seam App (§3.4). Abstrae local FS y S3 detrás de una interfaz uniforme. Cuando un
archivo se sube, seam-filer lo registra automáticamente en thumble.

### 3.2 Repositorio y estructura

```
github.com/ha1tch/seam-filer/
  cmd/seam-filer/
    main.go
  internal/
    api/
      handler.go      — HTTP handler principal
      fijuggler.go    — los diez endpoints del contrato fijuggler
      info.go         — endpoints adicionales de integración (§3.4)
      proxy.go        — helpers para streaming de archivos
    storage/
      storage.go      — interfaz Storage
      local.go        — backend local FS
      s3.go           — backend S3
    thumble/
      client.go       — cliente HTTP de thumble
      register.go     — flujo de registro + eager rendering
    config/
      config.go       — parse de TOML
  seam-filer.toml    — configuración por defecto
  Dockerfile
  Makefile
```

### 3.3 Contrato REST de fijuggler (diez endpoints)

seam-filer implementa exactamente el mismo contrato que el servidor Python de referencia
de fijuggler. Los endpoints son:

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/api/files/list` | Lista el contenido de un directorio. Query: `path`. |
| `GET` | `/api/files/get` | Descarga un archivo. Query: `path`. |
| `POST` | `/api/files/upload` | Sube un archivo. Multipart form: `path` + `file`. |
| `POST` | `/api/files/mkdir` | Crea un directorio. Body: `{path}`. |
| `POST` | `/api/files/rename` | Renombra o mueve. Body: `{from, to}`. |
| `POST` | `/api/files/delete` | Elimina archivo o directorio. Body: `{path}`. |
| `POST` | `/api/files/copy` | Copia archivo. Body: `{from, to}`. |
| `POST` | `/api/files/cut` | Mueve archivo. Body: `{from, to}`. |
| `POST` | `/api/files/paste` | Completa una operación cut anterior. Body: `{to}`. |
| `GET` | `/api/files/search` | Búsqueda por nombre. Query: `q`, `path`. |

Las rutas expuestas a fijuggler son siempre relativas a la raíz del tenant — el cliente
nunca ve rutas absolutas ni el prefijo de tenant_id (§3.9).

En local FS los directorios son reales. En S3 los directorios son simulados via prefijos
de clave (§3.9.3): `mkdir` no crea ningún objeto, `rename` reescribe prefijos de todos
los objetos bajo esa ruta, `delete` elimina todos los objetos con ese prefijo.

El endpoint `POST /api/files/delete` mueve al directorio `Trash/` en lugar de eliminar
inmediatamente (§3.9.2). La eliminación permanente se realiza via `POST /api/files/purge`.

### 3.4 Endpoints adicionales de integración

Estos endpoints no son parte del contrato fijuggler — los usa Seam App directamente:

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/api/info` | Metadata extendida. Query: `path`. Lee desde `file_record` en xolu. Devuelve mime_type, size_bytes, content_hash, thumble_asset_id, thumb_state, index_state, mtime. |
| `GET` | `/api/health` | Estado de seam-filer, xolu y thumble. |
| `GET` | `/api/capabilities` | Backends de storage activos, si thumble y xolu están configurados. |
| `POST` | `/api/files/purge` | Eliminación permanente desde Trash/. Body: `{path}`. Sólo para paths bajo Trash/. |

### 3.5 Flujo de upload

Cuando un archivo se sube via `POST /api/files/upload`, el flujo tiene dos fases:
una fase síncrona que completa antes de responder a fijuggler, y una fase asíncrona
que ocurre en background.

**Fase síncrona (bloqueante):**
```
1. seam-filer recibe el multipart
2. Determina ruta física: /{tenant_id}/{path} en storage
3. Escribe bytes en storage (local FS o S3)
4. Calcula SHA-256 del contenido (content_hash)
5. Detecta MIME type del contenido (no sólo de la extensión)
6. Crea file_record en xolu:
   {fijuggler_ref, tenant_id, content_hash, mime_type, size_bytes,
    thumb_state:"pending", index_state:"pending", summary_state:"skipped",
    locations:[{provider, path/key, tier:"hot", state:"active"}]}
7. Responde a fijuggler con listing estándar (nombre, tamaño, fecha)
   fijuggler nunca recibe thumble_asset_id ni xolu IDs
```

**Fase asíncrona (goroutines independientes, fire-and-forget):**
```
8. [goroutine A] Registrar en thumble: POST /assets {mode, path/key}
   → thumble responde {id, page_count, duration}
   → seam-filer actualiza file_record: thumble_asset_id, page_count, duration
   → si califica para eager rendering (§3.5.1): encola batch jobs en thumble
   → cuando jobs completan: actualiza file_record.thumb_state = "done"

9. [goroutine B] Encolar index_job en xolu para seam-indexer (§12.5)
   → seam-indexer extrae texto → actualiza file_record.index_state = "done"
```

La fase síncrona es deliberadamente mínima — escribe bytes y crea el file_record.
Si xolu no está disponible al momento del upload, seam-filer escribe el file_record
al WAL local (§3.7) y lo sincroniza cuando xolu vuelve.

El thumble_asset_id no se expone a fijuggler — sólo está disponible leyendo
`file_record` desde xolu via `GET /api/info`.

#### 3.5.1 Criterios de eager rendering

Un archivo califica para eager rendering si:

- El backend de thumble está configurado y alcanzable
- El formato es soportado por thumble (ver §6.1)
- Para documentos (PDF, Office, texto): `page_count ≤ eager_max_pages` (default: 100)
  Y `file_size_bytes ≤ eager_max_bytes` (default: 50 MB)
- Para video: `duration_seconds ≤ eager_max_duration` (default: 10800 = 3 horas)
- Para audio: siempre califica (el clip de 30s es barato de generar)
- Para imágenes: siempre califica (un solo thumbnail)

Los umbrales se configuran en `[thumble]` (§3.6).

### 3.6 Configuración (TOML)

```toml
[server]
addr              = ":8091"
read_timeout      = "30s"
write_timeout     = "120s"
idle_timeout      = "120s"
max_upload_bytes  = 524288000   # 500 MB

[storage]
backend = "local"               # "local" | "s3"
root    = "/var/seam-filer"    # raíz para backend local (ignorado si backend = "s3")

[xolu]
url       = "http://xolu:9090"
api_key   = ""
# WAL local para resiliencia cuando xolu no está disponible
wal_path  = "/var/seam-filer/wal.db"   # SQLite ligero; sólo pendientes de sync

[thumble]
url                = "http://thumble:8090"
api_key            = ""
eager_max_pages    = 100
eager_max_bytes    = 52428800            # 50 MB
eager_max_duration = 10800               # 3 horas en segundos
timeout            = "10s"

# Backend S3 — sólo relevante si storage.backend = "s3"
[[s3.bucket]]
name        = "seam-files"
endpoint    = "http://minio:9000"
region      = "us-east-1"
access_key  = "changeme"
secret_key  = "changeme"
use_tls     = false
root_prefix = "filer/"          # prefijo para todos los objetos de este bucket
```

### 3.7 `file_record` — fuente de verdad en xolu

No hay BoltDB. El estado de cada archivo vive en xolu como entidad `file_record`.
Es la fuente de verdad para todo el subsistema:

```go
type FileRecord struct {
    ID           int64              `xolu:"id"`
    TenantID     string             `xolu:"tenant_id"`
    FijugglerRef string             `xolu:"fijuggler_ref"` // path relativo al tenant root — estable
    ContentHash  string             `xolu:"content_hash"`  // SHA-256; cambio → re-registro thumble
    MimeType     string             `xolu:"mime_type"`     // detectado del contenido
    SizeBytes    int64              `xolu:"size_bytes"`
    PageCount    int                `xolu:"page_count"`    // 0 si no aplica
    Duration     int                `xolu:"duration"`      // segundos; 0 si no aplica
    ThumbleAssetID string           `xolu:"thumble_asset_id"`

    // Estado de procesamiento — dimensiones independientes
    ThumbState   string             `xolu:"thumb_state"`   // "pending"|"processing"|"done"|"failed"|"skipped"
    IndexState   string             `xolu:"index_state"`
    SummaryState string             `xolu:"summary_state"` // "skipped" por defecto hasta que LLM esté configurado

    // Ubicaciones físicas — lista para soportar migración entre providers
    Locations    []StorageLocation  `xolu:"locations"`

    CreatedAt    time.Time          `xolu:"created_at"`
    UpdatedAt    time.Time          `xolu:"updated_at"`
    LastVerified *time.Time         `xolu:"last_verified"` // última verificación de integridad
}

type StorageLocation struct {
    Provider   string     `xolu:"provider"`    // "local" | "s3" | "glacier" | ...
    Path       string     `xolu:"path"`        // path físico o S3 URI (s3://bucket/key)
    Tier       string     `xolu:"tier"`        // "hot" | "warm" | "cold"
    State      string     `xolu:"state"`       // "active" | "migrating" | "archived" | "evicted"
    SizeBytes  int64      `xolu:"size_bytes"`  // puede diferir del original si comprimido
    StoredAt   time.Time  `xolu:"stored_at"`
    VerifiedAt *time.Time `xolu:"verified_at"`
}
```

El campo `FijugglerRef` es el identificador estable del archivo desde la perspectiva
del usuario: `Documents/invoices/2026/invoice123.pdf`. Es independiente del provider
de storage — si el archivo migra de local FS a S3, el `FijugglerRef` no cambia.
El `Path` en `StorageLocation` sí cambia.

**`file_record` está indexado por `fijuggler_ref` en xolu** para que las búsquedas
sean un escaneo indexado O(log n), no un full scan.

#### WAL local para resiliencia

Si xolu no está disponible cuando seam-filer procesa un upload, el file_record se
escribe en un WAL local (SQLite ligero, sólo el schema mínimo necesario para el sync):

```go
type WALEntry struct {
    ID           int64
    FijugglerRef string
    TenantID     string
    Payload      string    // file_record serializado como JSON
    Synced       bool
    CreatedAt    time.Time
    AttemptedAt  *time.Time
}
```

Un goroutine de sync en background drena el WAL periódicamente (cada 30 segundos o
al detectar que xolu volvió). Una vez sincronizado, `WALEntry.Synced = true`. El WAL
se compacta eliminando entradas sincronizadas mayores de 24 horas.

El WAL es un mecanismo de delivery, no un store paralelo. Una vez sincronizado, xolu
es la única fuente de verdad.

### 3.8 Multi-tenancy

El tenant_id se pasa como header `X-Seam-Tenant` desde el proxy de Seam App.
seam-filer prefija todas las operaciones de storage con el tenant_id. fijuggler
ve una raíz limpia sin el prefijo de tenant — la jerarquía que ve el usuario
siempre comienza desde `/`.

Para S3, el tenant_id se incorpora al prefijo de clave: `filer/{tenant_id}/path`.

### 3.9 Jerarquía de directorios

#### 3.9.1 Estructura de la raíz del tenant

Al provisionar un tenant, seam-filer crea los siguientes directorios bien conocidos:

```
/                           ← raíz del tenant (fijuggler lo muestra como "My Files")
  Inbox/                    ← directorio de aterrizaje por defecto para uploads
  Trash/                    ← papelera; archivos movidos aquí por delete (§3.9.2)
```

Todo lo demás es libre — el usuario crea la jerarquía que necesita. Seam AMS no
prescribe ninguna estructura por debajo de los directorios bien conocidos. La
organización documental se hace via metadatos en el módulo Documental (tipo,
tags, descripción), no via estructura de directorios.

La raíz `/` en la representación física es `/{tenant_id}/` — invisible para fijuggler.

#### 3.9.2 Semántica de `delete` y `purge`

`POST /api/files/delete` nunca destruye un archivo inmediatamente. Mueve el archivo
(o el directorio completo) a `Trash/`:

```
/Documents/invoices/2026/invoice123.pdf
  → /Trash/Documents/invoices/2026/invoice123.pdf
```

La ruta original se preserva bajo `Trash/` para facilitar la restauración. El
`fijuggler_ref` en `file_record` se actualiza al nuevo path bajo `Trash/`.

`POST /api/files/purge` elimina permanentemente. Sólo acepta paths bajo `Trash/`.
Elimina el archivo del storage, marca el `file_record` como eliminado en xolu
(soft-delete — `deleted_at` timestamp), y elimina los derivados de thumble.

El módulo Documental y seam-indexer ignoran archivos bajo `Trash/` para indexación
y thumbnailing. El tray de formularios ignora documentos cuyos `fijuggler_ref`
apuntan a `Trash/`.

**Retención automática:** un job programado en seam-filer puede configurarse para
purgar automáticamente archivos en `Trash/` más antiguos de N días (configurable,
default 30). Deshabilitado por defecto en el MVP.

#### 3.9.3 Directorios en S3

S3 no tiene directorios reales. Las operaciones de directorio se implementan así:

| Operación | Implementación en S3 |
|-----------|---------------------|
| `list ?path=/Documents/` | ListObjects con prefix `{tenant_id}/Documents/`, delimiter `/` |
| `mkdir /Documents/2026/` | No-op — los "directorios" existen sólo si tienen contenido |
| `rename /a/ → /b/` | CopyObject + DeleteObject para cada objeto con prefix `{tenant_id}/a/` |
| `delete /Documents/` | CopyObject de cada objeto a `{tenant_id}/Trash/Documents/` + Delete |
| `search ?q=invoice` | ListObjects sin delimiter, filtrar por nombre localmente |

La simulación de directorios es transparente para fijuggler. Un directorio vacío en S3
no puede existir — si el usuario crea una carpeta vacía y no sube nada en ella, la
carpeta desaparece. seam-filer puede mantener un objeto placeholder (`{path}/.keep`)
para preservar directorios vacíos si se configura `preserve_empty_dirs = true`.

#### 3.9.4 `fijuggler_ref` — identificador estable

El `fijuggler_ref` almacenado en `file_record` y en los documentos es siempre la
ruta tal como fijuggler la ve: relativa al tenant root, sin el tenant_id, comenzando
sin slash inicial:

```
Documents/invoices/2026/invoice123.pdf   ✓
/Documents/invoices/2026/invoice123.pdf  ✗  (no incluye slash inicial)
tenant-abc/Documents/invoices/...        ✗  (no incluye tenant_id)
/var/seam-filer/tenant-abc/...          ✗  (no es path absoluto)
```

La traducción de `fijuggler_ref` a path físico es determinista y no requiere lookup:

```go
// Local FS:
physicalPath = filepath.Join(storage.root, tenantID, fijugglerRef)

// S3:
s3Key = rootPrefix + tenantID + "/" + fijugglerRef
```

Si el archivo migra de local FS a S3, el `fijuggler_ref` no cambia. Sólo cambia
el `StorageLocation.Path` en `file_record`. Todos los documentos que referencian
ese archivo via `fijuggler_ref` siguen funcionando sin actualización.

---

## 4. thumble — Servicio de Previsualizaciones

### 4.1 Estado actual (v0.4.0)

thumble v0.4.0 es un servicio Go con los siguientes endpoints:

| Método | Ruta | Descripción |
|--------|------|-------------|
| `POST` | `/assets` | Registra un archivo (local o S3). Devuelve asset_id. |
| `POST` | `/assets/upload` | Sube y registra en un paso. |
| `GET` | `/assets/{id}` | Metadata del asset. |
| `DELETE` | `/assets/{id}` | Elimina asset y sus derivados. |
| `GET` | `/assets/{id}/{profile}` | Devuelve el derivado (thumb/preview). Genera si no existe. |
| `POST` | `/assets/{id}/jobs` | Crea un job de renderizado. |
| `GET` | `/jobs/{id}` | Estado de un job. |
| `DELETE` | `/jobs/{id}` | Cancela un job. |
| `GET` | `/capabilities` | Renderers disponibles. |
| `GET` | `/health` | Estado del servicio. |

El `Asset` registrado en thumble tiene: `ID`, `Mode` (local/s3/upload), `Path`,
`Bucket`, `Key`, `Extension`, `CreatedAt`, `UpdatedAt`. Notablemente, **no tiene**
`PageCount`, `Duration`, ni ningún campo de metadata de contenido.

### 4.2 Extensiones requeridas

Para soportar el flujo de Seam Media, thumble necesita las siguientes adiciones.
Estas son cambios en thumble, no en seam-filer ni en Seam App:

#### 4.2.1 `page_count` y `duration` en el response de registro

**Cambio:** Al registrar un asset (`POST /assets`), thumble debe probar el archivo
y devolver metadata de contenido en el response:

```json
{
  "id": "abc123",
  "mode": "local",
  "extension": "pdf",
  "size": 204800,
  "page_count": 12,
  "duration": 0,
  "mtime": "...",
  "created_at": "..."
}
```

- `page_count`: para PDF usa `pdfinfo -l` (ya disponible via pdftoppm). Para Office
  usa `soffice --headless --cat` o equivalente para contar páginas. Para otros formatos: 0.
- `duration`: para audio/video usa `ffprobe -v quiet -show_entries format=duration`.
  Para otros formatos: 0.

Ambos campos también se exponen en `GET /assets/{id}`.

#### 4.2.2 Job batch endpoint

**Nuevo endpoint:** `POST /assets/{id}/jobs/batch`

Request:
```json
{
  "jobs": [
    {"profile": "thumb", "page": 1, "sq": 256, "fmt": "jpeg", "q": 75},
    {"profile": "thumb", "page": 2, "sq": 256, "fmt": "jpeg", "q": 75},
    ...
  ]
}
```

Response `202 Accepted`:
```json
{
  "batch_id": "batch_xyz",
  "total": 12,
  "status": "pending"
}
```

**Nuevo endpoint:** `GET /batch-jobs/{batch_id}`

Response:
```json
{
  "batch_id": "batch_xyz",
  "total": 12,
  "done": 8,
  "failed": 0,
  "status": "running"   // "pending" | "running" | "done" | "failed"
}
```

seam-filer usa el batch endpoint para el eager rendering. Seam App sondea
`GET /batch-jobs/{batch_id}` y actualiza el documento xolu cuando `status = "done"`.

#### 4.2.3 Nuevo tipo de job: audio clip

**Nuevo `profile`:** `"audioclip"` en `POST /assets/{id}/jobs`

Request:
```json
{
  "profile": "audioclip",
  "duration": 30,
  "fade_in":  0.5,
  "fade_out": 2.0,
  "bitrate":  "64k",
  "channels": 1
}
```

Implementación: `ffmpeg -i {input} -t {duration} -ac {channels} -af "afade=t=in:st=0:d={fade_in},afade=t=out:st={duration-fade_out}:d={fade_out}" -c:a libvorbis -b:a {bitrate} {output}.ogg`

El derivado se sirve via `GET /assets/{id}/audioclip`.

#### 4.2.4 Nuevo tipo de job: frame de video

**Nuevo `profile`:** `"frame"` en `POST /assets/{id}/jobs`

Request:
```json
{
  "profile": "frame",
  "t":       "00:01:30",   // timestamp — mismo formato que el campo "t" existente
  "w":       1280,
  "h":       720,
  "fmt":     "jpeg",
  "q":       80
}
```

El campo `t` ya existe en `jobRequest` en thumble v0.4.0 — sólo necesita ser
habilitado para el nuevo profile `"frame"`. El derivado se identifica por asset_id
+ profile `"frame"` + timestamp, permitiendo cachear frames individuales.

#### 4.2.5 Config additions

```toml
[eager]
max_pages    = 100
max_bytes    = 52428800   # 50 MB
max_duration = 10800      # 3 horas
```

---

## 5. Algoritmos de Renderizado

### 5.1 Documentos de texto y oficina (PDF, DOCX, ODT, XLSX, ODS, PPTX, MD, TXT)

Al vincular un archivo de documento, seam-filer envía al batch endpoint de thumble
`page_count` jobs del perfil `thumb` con calidad 75, formato JPEG, tamaño 1200×(auto):

```go
func documentJobs(pageCount int) []jobRequest {
    jobs := make([]jobRequest, pageCount)
    for i := range jobs {
        jobs[i] = jobRequest{
            Profile: "thumb",
            Page:    i + 1,
            Width:   1200,
            Format:  "jpeg",
            Quality: 75,
        }
    }
    return jobs
}
```

Todos los documentos — PDF, Office, texto, markdown — comparten el mismo viewer
de página en el módulo Documental. El JPEG de cada página se recupera via
`GET /assets/{thumble_asset_id}/thumb?page={n}`.

### 5.2 Imágenes

Un job de thumbnail (256px square, WebP) para el pill del tray.
Un job de preview (1200px ancho, contiene, JPEG q85) para el visor.

### 5.3 Audio — clip de preview

Un job `audioclip` con:
- `duration`: 30 segundos
- `fade_in`: 0.5 segundos
- `fade_out`: 2.0 segundos
- `bitrate`: "64k"
- `channels`: 1 (mono)

Un job adicional de waveform (800×120px, WebP) para visualización.

El clip resultante es aproximadamente 240 KB para 30 segundos a 64kbps mono.

### 5.4 Video — frame sampling adaptativo

El algoritmo es determinístico: dado `duration_seconds`, calcula `n_frames`
timestamps a los que extraer frames.

```go
func videoFrameTimestamps(durationSeconds int) []int {
    const (
        targetFrames = 20
        minInterval  = 30   // segundos
        maxFrames    = 30
    )

    if durationSeconds <= 0 {
        return nil
    }

    interval := durationSeconds / targetFrames
    if interval < minInterval {
        interval = minInterval
    }

    n := durationSeconds / interval
    if n > maxFrames {
        n = maxFrames
    }

    timestamps := make([]int, n)
    for i := range timestamps {
        timestamps[i] = (i + 1) * interval
    }
    return timestamps
}
```

Comportamiento resultante:

| Duración | Intervalo | Frames |
|----------|-----------|--------|
| < 10 min | 30 s | hasta 20 |
| 10–30 min | ~1 min | 10–30 |
| 30–60 min | ~2 min | 15–30 |
| 60–150 min | ~5 min | 12–30 |
| > 150 min | 5 min, cap 30 | 30 |

seam-filer envía `n` jobs de profile `"frame"` al batch endpoint de thumble,
uno por timestamp. Los `VideoFrame` resultantes se almacenan en el documento xolu.

---

## 6. Formatos Soportados

### 6.1 Tabla de formatos por renderer

| Grupo | Extensiones | Renderer | Page count | Eager |
|-------|-------------|----------|------------|-------|
| PDF | pdf | pdftoppm | `pdfinfo -l` | Sí, si ≤ max_pages |
| Office | docx, doc, xlsx, xls, pptx, ppt, odt, ods, odp, rtf, wpd, wps, sxw, sxc, sxi y variantes | soffice | soffice headless | Sí, si ≤ max_pages |
| Texto/Markup | md, markdown, rst, tex, epub, txt | pandoc | conteo de páginas pandoc | Sí, si ≤ max_pages |
| Imagen | jpg, jpeg, png, gif, tiff, bmp, webp | imaging (go) | 1 | Siempre |
| Imagen avanzada | psd, heic, heif, eps, svg, ai, ico, xcf, cr2, nef, arw, dng | magick | 1 | Siempre |
| Video | mp4, mkv, avi, mov, webm, flv, wmv, ts, m4v, 3gp, ogv | ffmpeg | — | Sí, si ≤ max_duration |
| Audio | mp3, flac, wav, ogg, aac, m4a, opus | ffmpeg | — | Siempre |
| Web | html, htm | chromium | 1 | Sí |

Formatos no listados: sin renderizado. El visor muestra icono MIME + metadata + botón
de descarga (si permiso `Full`).

---

## 7. seam-media — Consolidación Futura

### 7.1 Motivación

seam-filer, thumble y fijuggler son tres repositorios independientes que siempre se
despliegan juntos en el contexto de Seam AMS. La consolidación en un repositorio
`seam-media` simplifica:

- Configuración unificada (un solo TOML)
- Health endpoint agregado
- Despliegue como stack único (docker-compose)
- Integración directa fijuggler ↔ thumble (thumbnails en el panel de preview de fijuggler)
- Vendoring de fijuggler dentro del bundle de Seam App gestionado desde un punto

### 7.2 Composición del repositorio seam-media

```
github.com/ha1tch/seam-media/
  cmd/
    seam-filer/            — el servicio (igual al repo actual)
  vendor/
    fijuggler/              — fijuggler como submódulo o vendored bundle
  docker/
    docker-compose.yml      — stack completo: seam-filer + thumble + minio (dev)
    thumble.toml            — config de thumble para el stack
    seam-filer.toml        — config de seam-filer para el stack
  SEAM_MEDIA_SPEC.md       — este documento (relocado)
```

thumble mantiene su propio repositorio — no es subido a seam-media porque otros
proyectos (canvas-md, quag) lo usan independientemente. seam-media lo referencia
via URL en su docker-compose.

### 7.3 Integración fijuggler ↔ thumble (pendiente)

La integración directa que actualmente no existe: cuando fijuggler lista un directorio,
el panel de preview podría mostrar thumbnails de thumble en lugar de íconos genéricos.

Esto requiere:
1. fijuggler recibe una `thumbUrl` por archivo en el response del listing endpoint
2. seam-filer incluye la `thumb_url` de thumble en el response de `GET /api/files/list`
   (campo adicional, opcional — fijuggler lo usa si está presente, ignora si no)
3. thumble debe haber procesado el archivo previamente (eager rendering via seam-filer)

Esta integración no bloquea el MVP. fijuggler muestra íconos genéricos hasta que se
implemente.

---

## 8. Integración con Seam App

### 8.1 Proxy `/api/files/*`

Seam App expone un proxy transparente que reenvía todas las llamadas de fijuggler
a seam-filer:

```go
// internal/handlers/api/files.go

func (h *FilesHandler) Proxy(w http.ResponseWriter, r *http.Request) {
    if h.media.IsStub() {
        http.Error(w, "seam-media not configured", http.StatusServiceUnavailable)
        return
    }

    // Strip /api/files prefix, add tenant header, forward
    path := strings.TrimPrefix(r.URL.Path, "/api/files")
    target := h.media.FilerURL() + "/api/files" + path
    if r.URL.RawQuery != "" {
        target += "?" + r.URL.RawQuery
    }

    req, _ := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
    req.Header.Set("X-Seam-Tenant", h.tenantFromRequest(r))
    if h.media.APIKey() != "" {
        req.Header.Set("Authorization", "Bearer "+h.media.APIKey())
    }
    // Copy relevant headers from original request
    for _, h := range []string{"Content-Type", "Content-Length"} {
        if v := r.Header.Get(h); v != "" {
            req.Header.Set(h, v)
        }
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        http.Error(w, "upstream error", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}
```

La ruta en el router de Seam App:

```go
r.Get("/api/files/*", auth.Required, handlers.Files.Proxy)
r.Get("/files", handlers.Files.Page)
```

### 8.2 Ruta `/files`

La página que monta `<fj-app>`:

```go
func (h *FilesHandler) Page(w http.ResponseWriter, r *http.Request) {
    if h.media.IsStub() {
        // Render a placeholder page explaining seam-media is not configured
        h.renderStubPage(w, r)
        return
    }
    h.render(w, r, "files", nil)
}
```

### 8.3 Uso en el módulo Documental

El módulo Documental usa `media.Client` (§9) para:

1. `FileMetadata(ctx, fijuggler_ref)` → obtiene mime_type, size_bytes, thumble_asset_id
2. `ThumbleAssetID(ctx, fijuggler_ref)` → shortcut para obtener solo el ID de thumble
3. `FileContentURL(ctx, fijuggler_ref)` → URL firmada o proxy URL para descarga
   (controlada por permiso `Full` en la capa de Seam App)

El módulo Documental NO llama thumble directamente. Toda interacción con thumble pasa
por seam-filer, que tiene el ThumbleAssetID en su metadata store.

Para el visor de documentos, Seam App hace redirect a thumble directamente para servir
derivados (JPEGs de páginas, frames de video), pasando por su propio proxy de autenticación
si thumble requiere API key.

---

## 9. Interfaz `media.Client` — Contrato de Integración

La interfaz Go que Seam App define y que seam-media implementa cuando se integra:

```go
// Package media define el contrato entre Seam App y el subsistema Seam Media.
// En ausencia de seam-media, NewStub() provee una implementación que retorna
// ErrNotConfigured para todas las operaciones.
package media

import (
    "context"
    "errors"
    "time"
)

// ErrNotConfigured se retorna cuando seam-media no está configurado.
var ErrNotConfigured = errors.New("seam-media not configured")

// FileMetadata describe un archivo en seam-filer.
type FileMetadata struct {
    Path           string    // ruta relativa en seam-filer
    MimeType       string    // detectado del contenido
    SizeBytes      int64
    ContentHash    string    // SHA-256 hex
    ThumbleAssetID string    // ID en thumble; vacío si no registrado
    ModifiedAt     time.Time
}

// Client es la interfaz que usa Seam App para interactuar con Seam Media.
type Client interface {
    // IsStub retorna true si esta instancia es el stub sin configuración.
    IsStub() bool

    // FilerURL retorna la URL base de seam-filer (para el proxy).
    FilerURL() string

    // APIKey retorna el API key para autenticar requests a seam-filer.
    APIKey() string

    // FileMetadata retorna metadata extendida de un archivo en seam-filer.
    // El fijuggler_ref es la ruta relativa del archivo tal como fijuggler la conoce.
    FileMetadata(ctx context.Context, ref string) (*FileMetadata, error)

    // ThumbleAssetID retorna el ID de thumble para un archivo dado su fijuggler_ref.
    // Retorna "" si el archivo no ha sido registrado con thumble aún.
    ThumbleAssetID(ctx context.Context, ref string) (string, error)

    // FileContentURL retorna la URL para acceder al contenido del archivo.
    // Puede ser una URL directa a seam-filer o una URL firmada para S3.
    FileContentURL(ctx context.Context, ref string) (string, error)
}

// stubClient implementa Client retornando ErrNotConfigured.
type stubClient struct{}

func (s *stubClient) IsStub() bool                                           { return true }
func (s *stubClient) FilerURL() string                                       { return "" }
func (s *stubClient) APIKey() string                                         { return "" }
func (s *stubClient) FileMetadata(_ context.Context, _ string) (*FileMetadata, error) {
    return nil, ErrNotConfigured
}
func (s *stubClient) ThumbleAssetID(_ context.Context, _ string) (string, error) {
    return "", ErrNotConfigured
}
func (s *stubClient) FileContentURL(_ context.Context, _ string) (string, error) {
    return "", ErrNotConfigured
}

// NewStub retorna un Client que reporta ErrNotConfigured para todas las operaciones.
// Es el valor por defecto en Seam App cuando seam-media no está configurado.
func NewStub() Client { return &stubClient{} }

// NewHTTPClient retorna un Client real que habla con seam-filer via HTTP.
// Se usa cuando seam-media está desplegado y configurado.
func NewHTTPClient(filerURL, apiKey string) Client {
    return &httpClient{filerURL: filerURL, apiKey: apiKey}
}
```

La implementación `httpClient` vive en `internal/media/http.go` y se añade cuando
seam-media está disponible. El stub es suficiente para todo el desarrollo del módulo
Documental.

---

## 10. Despliegue y Configuración de Seam App

### 10.1 Variables de entorno / config de Seam App

```toml
[media]
enabled   = false           # true cuando seam-media está desplegado
filer_url = "http://seam-filer:8091"
api_key   = ""
```

Cuando `enabled = false`, `media.NewStub()` se usa en lugar de `media.NewHTTPClient()`.

### 10.2 Demo packaging (START.cmd / health-check)

Cuando seam-media se integra en el demo packaging:

```
START.cmd arranca:
  1. xolu
  2. thumble
  3. seam-filer
  4. seam-app

Health-check loop verifica:
  - GET http://localhost:9090/health  (xolu)
  - GET http://localhost:8090/health  (thumble)
  - GET http://localhost:8091/api/health  (seam-filer)
  - GET http://localhost:8080/health  (seam-app)

Todos deben responder 200 antes de abrir el browser.
```

### 10.3 Modo air-gap (sin Internet, sin S3)

seam-filer opera perfectamente con `storage.backend = "local"` y thumble configurado
con `allow_path_mode = true`. Ninguna dependencia de red externa. Válido para
instalaciones on-premise con storage NAS montado localmente.

---

## 11. Riesgos Conocidos y Decisiones Pendientes

### 11.1 Consistencia seam-filer ↔ thumble al modificar archivos

**Riesgo:** Si un archivo se sobreescribe en seam-filer (via `POST /api/files/upload`
con el mismo path), el `thumble_asset_id` anterior queda asociado al contenido antiguo
en el cache de thumble. Los derivados servidos serán incorrectos hasta que thumble
invalide su cache por firma de archivo.

**Mitigación:** thumble usa `FileSig` (mtime + size) para validar su cache en cada
request. Si el archivo cambió, thumble re-renderiza automáticamente. seam-filer
re-registra el archivo con thumble si detecta que `content_hash` cambió, obteniendo
un nuevo `thumble_asset_id` y actualizando el meta store.

**Decisión pendiente:** Definir si seam-filer debe notificar a Seam App (via NATS)
cuando un archivo cambia, para que el módulo Documental invalide el `thumb_url` y
`thumble_asset_id` almacenados en xolu. Recomendado cuando NATS esté integrado.

### 11.2 Rendimiento del eager rendering en uploads masivos

**Riesgo:** Si el usuario sube múltiples documentos PDF grandes simultáneamente,
seam-filer puede saturar la cola de jobs de thumble, degradando el tiempo de respuesta
para otros renders.

**Mitigación:** El eager rendering se hace en goroutines background desacopladas del
request de upload. La cola de thumble tiene `queue_depth` configurable. Para uploads
masivos, el eager rendering puede deshabilitarse temporalmente via config sin afectar
la funcionalidad (el rendering ocurre on-demand en el primer acceso).

### 11.3 WAL y consistencia con xolu

**Riesgo:** El WAL local garantiza que los file_records se sincronizan con xolu
eventualmente, pero durante la ventana de outage de xolu el resto del sistema
(módulo Documental, seam-indexer, búsqueda) no puede ver los archivos recién
subidos.

**Mitigación:** El WAL es una solución de resiliencia para outages breves. En
despliegues donde la disponibilidad del file manager durante outages de xolu es
crítica, seam-filer puede configurarse con `xolu.required = false` — en ese modo
el upload responde con éxito incluso sin xolu, y el WAL sincroniza cuando xolu
vuelve. Con `xolu.required = true` (default), el upload falla si el file_record no
puede crearse en xolu ni en el WAL.

**Riesgo adicional:** En despliegues con múltiples instancias de seam-filer
(horizontal scaling), el WAL es local a cada instancia. El xolu file_record es
compartido correctamente, pero los retries del WAL pueden crear duplicados si
dos instancias procesan el mismo entry. Mitigación: usar el `content_hash` como
idempotency key al crear file_records en xolu — xolu rechaza duplicados con 409.

### 11.4 Autenticación de seam-filer

**Riesgo:** seam-filer expone operaciones destructivas (delete, rename) que deben
estar protegidas. En el MVP, la seguridad depende de que seam-filer sólo sea
alcanzable desde Seam App (no expuesto a la red pública).

**Mitigación MVP:** seam-filer en red interna, autenticación via API key en el header
`Authorization: Bearer {api_key}`. Seam App verifica el usuario de Seam AMS antes
de forwarded al proxy — seam-filer confía en que si el request llega, ya fue
autenticado por Seam App.

**Decisión pendiente:** Para despliegues donde seam-filer necesite estar expuesto,
implementar JWT con los mismos roles de Seam AMS. No requerido para el MVP.

---


## 12. seam-indexer — Servicio de Extracción de Texto

### 12.1 Propósito

seam-indexer extrae el contenido de texto de los archivos almacenados en seam-filer
y lo indexa para búsqueda de texto completo. Es el primer consumidor de una cola de
jobs que en el futuro también alimentará el pipeline de resúmenes con LLM.

El indexer no conoce documentos, activos ni ninguna entidad de negocio de Seam AMS.
Su único insumo es un archivo en seam-filer (referenciado por `fijuggler_ref`) y su
único output es el texto extraído, almacenado en xolu como entidad `file_index`.

### 12.2 Principio de diseño

**El archivo existe antes que el documento.** La indexación se dispara inmediatamente
al subir un archivo a seam-filer — en el mismo momento que thumble recibe el registro.
El documento que eventualmente se cree y vincule al archivo encontrará el texto ya
extraído. Ninguna operación de vinculación, creación de documento ni adjunto tiene
relevancia para cuándo se indexa un archivo.

### 12.3 Entidad `index_job` en xolu

La cola de indexación es una entidad en xolu, usando xolu como store de jobs:

```go
type IndexJob struct {
    ID           int64     `xolu:"id"`
    TenantID     string    `xolu:"tenant_id"`
    FijugglerRef string    `xolu:"fijuggler_ref"`  // ruta relativa en seam-filer
    MimeType     string    `xolu:"mime_type"`       // para decidir qué extractor usar
    JobType      string    `xolu:"job_type"`        // "extract" | "summarise" (futuro)
    Status       string    `xolu:"status"`          // "pending" | "processing" | "done" | "failed"
    Attempts     int       `xolu:"attempts"`
    LastError    string    `xolu:"last_error"`
    CreatedAt    time.Time `xolu:"created_at"`
    UpdatedAt    time.Time `xolu:"updated_at"`
}
```

El campo `job_type` permite que en el futuro el mismo worker loop procese distintos
tipos de jobs. Para el MVP sólo existe el tipo `"extract"`.

### 12.4 Entidad `file_index` en xolu

El texto extraído vive en una entidad separada en xolu, keyed por `fijuggler_ref`:

```go
type FileIndex struct {
    ID           int64     `xolu:"id"`
    TenantID     string    `xolu:"tenant_id"`
    FijugglerRef string    `xolu:"fijuggler_ref"`  // mismo ref que en seam-filer
    FullText     string    `xolu:"full_text"`       // texto extraído, para FTS5
    WordCount    int       `xolu:"word_count"`      // útil para diagnóstico
    ExtractedAt  time.Time `xolu:"extracted_at"`
    ExtractedBy  string    `xolu:"extracted_by"`    // versión del extractor
}
```

`full_text` está cubierto por un índice FTS5 en xolu. La búsqueda compuesta del módulo
Documental (§5.6 de DOCUMENTS_MODULE_SPEC) une `document.fijuggler_ref` con
`file_index.fijuggler_ref` para obtener el score FTS.

### 12.5 Quién encola jobs

seam-filer encola un `index_job` de tipo `"extract"` inmediatamente después de un
upload exitoso, en el mismo flujo que registra el archivo con thumble:

```
POST /api/files/upload → seam-filer:
  1. Escribe archivo en storage
  2. Calcula content_hash
  3. Detecta MIME type
  4. Registra en thumble (§3.5)
  5. Encola index_job en xolu:
     {tenant_id, fijuggler_ref, mime_type, job_type:"extract", status:"pending"}
  6. Responde a fijuggler
```

Los pasos 4 y 5 son fire-and-forget en goroutines separadas — no bloquean la respuesta
al cliente.

### 12.6 Worker de extracción

El indexer es un proceso Go simple con un loop de polling:

```
loop cada 5 segundos:
  1. OQL: SELECT TOP 10 * FROM index_jobs
          WHERE status = 'pending' AND job_type = 'extract'
          ORDER BY created_at
  2. Para cada job:
     a. Actualizar status → 'processing'
     b. Llamar seam-filer GET /api/files/get?path={fijuggler_ref}
        → obtener bytes del archivo
     c. Extraer texto según mime_type (§12.7)
     d. Si extracción exitosa:
          - Crear/actualizar file_index en xolu con full_text
          - Actualizar index_job status → 'done'
     e. Si extracción falla:
          - Incrementar attempts
          - Si attempts >= 3: status → 'failed', registrar last_error
          - Si attempts < 3: status → 'pending' (reintento en próximo ciclo)
```

El indexer no mantiene estado local. Toda la persistencia va a xolu. Si el indexer
se reinicia, retoma los jobs `pending` desde donde quedó.

### 12.7 Extractores de texto

| MIME / Extensión | Extractor | Método |
|------------------|-----------|--------|
| `application/pdf` | pdftotext | `pdftotext -layout -enc UTF-8 - -` (stdin→stdout) |
| Office (docx, xlsx, pptx, odt, ods, odp) | soffice | `soffice --headless --convert-to txt --outdir /tmp` |
| `text/plain`, `text/csv` | directo | leer bytes como UTF-8 |
| `text/markdown`, `text/x-markdown` | directo | leer bytes, strip de sintaxis MD básica |
| `text/html` | pandoc | `pandoc -f html -t plain` |
| Resto | ninguno | job marcado `done` con `full_text = ""` |

Los mismos binarios que thumble ya tiene instalados (pdftotext vía poppler, soffice,
pandoc) — seam-indexer no requiere nuevas dependencias si se co-despliega con thumble.

### 12.8 Repositorio y estructura

```
github.com/ha1tch/seam-indexer/
  cmd/seam-indexer/
    main.go
  internal/
    worker/
      worker.go       — loop de polling y despacho
      extract.go      — lógica de extracción por tipo
    extractor/
      pdf.go          — pdftotext
      office.go       — soffice
      plain.go        — txt, md, csv
      html.go         — pandoc
    config/
      config.go
    xolu/
      client.go       — cliente xolu (reutiliza patrón del cliente en seam-app)
  seam-indexer.toml
  Dockerfile
```

### 12.9 Configuración

```toml
[server]
poll_interval = "5s"    # frecuencia de polling de la cola
batch_size    = 10      # jobs por ciclo
max_attempts  = 3

[xolu]
url       = "http://xolu:9090"
tenant_id = ""          # vacío = no-tenant mode; set para tenant único

[filer]
url     = "http://seam-filer:8091"
api_key = ""

[extractors]
pdftotext_bin  = "pdftotext"
soffice_bin    = "soffice"
pandoc_bin     = "pandoc"
timeout        = "60s"
```

### 12.10 Extensión futura: job tipo `"summarise"`

Cuando se agregue el pipeline de LLM:

1. El worker detecta jobs `job_type = "summarise"` en el mismo loop
2. Prerequisito: `file_index.full_text` debe existir (el job `"extract"` debe estar `done`)
3. El worker envía `full_text` al LLM API configurado
4. El resultado se almacena en `file_index.ai_summary`
5. El módulo Documental incluye `ai_summary` como quinta señal en el score de búsqueda

No se requiere ningún cambio en la cola, en el schema de `index_job`, ni en el worker
loop para agregar este tipo. Sólo se añade un nuevo case en el despacho de tipos y un
campo en `file_index`.

### 12.11 Despliegue en el stack de Seam Media

```yaml
# docker-compose addition
seam-indexer:
  image: ghcr.io/ha1tch/seam-indexer:latest
  depends_on:
    - xolu
    - seam-filer
  volumes:
    - ./seam-indexer.toml:/etc/seam-indexer/seam-indexer.toml
```

Health-check en `START.cmd`:

```
GET http://localhost:8092/health  (seam-indexer)
```

seam-indexer expone `/health` que verifica alcanzabilidad de xolu y seam-filer.

---

*Documento producido en sesión de diseño de arquitectura de Seam AMS, Mayo 2026.*  
*Copyright (c) 2026 haitch — h@ual.li*
