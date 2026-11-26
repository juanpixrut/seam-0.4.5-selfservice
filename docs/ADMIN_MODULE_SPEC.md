# Seam AMS — Módulo Admin
## Especificación de Diseño

**Versión:** 0.1  
**Fecha:** Mayo 2026  
**Estado:** Borrador interno  
**Repositorio:** github.com/ha1tch/seam (módulo `admin`)

---

## 1. Propósito y Alcance

El módulo Admin es la superficie de administración del sistema. Es donde los usuarios
privilegiados configuran, diseñan, y verifican todos los aspectos del sistema antes de
que sean visibles para los usuarios operacionales.

A diferencia de los módulos operacionales (`assets`, `masters`, `monitoring`, `reports`),
el módulo Admin no produce ni consume datos de negocio — administra los metadatos que
definen cómo los otros módulos se comportan.

### 1.1 Perfiles de usuario del módulo Admin

El módulo Admin tiene dos perfiles de uso distintos que acceden a la misma herramienta:

**Operador DIY** — el cliente que instala Seam AMS y lo configura autónomamente. Necesita
una interfaz guiada, lenguaje no técnico, validaciones inmediatas, y feedback visual en
tiempo real. No debe necesitar editar JSON directamente para tareas comunes.

**Consultor Seam AMS** — un profesional que configura el sistema en nombre del cliente
en un engagement de consultoría. Puede acceder a la misma herramienta pero en modo
avanzado, con acceso directo al JSON del schema, herramientas de diagnóstico adicionales,
y capacidad de aplicar cambios en lote.

Estos dos perfiles se distinguen por rol (`admin` vs `seam_consultant`) y el módulo
expone capacidades adicionales para el rol consultor sin una interfaz completamente
separada.

### 1.2 Dependencias del módulo

El módulo Admin depende en este orden:

```
core (formengine, i18n, listing, auth, xolu)
  ↓
masters (para enumerar entidades del sistema en form designer)
  ↓
admin
```

No depende de `assets`, `monitoring`, ni `reports`. Puede compilarse y desplegarse sin
ellos.

### 1.3 Extracción desde el módulo principal

Las siguientes rutas y handlers se extraen del módulo principal al módulo Admin:

| Ruta actual | Nuevo módulo | Notas |
|-------------|-------------|-------|
| `/settings/*` | `admin` | Configuración general, notificaciones, features |
| `/admin/users/*` | `admin` | Gestión de usuarios — código existente |
| `/fsm/*` (editor write) | `admin` | El viewer de sólo lectura puede permanecer en `assets` |
| `/asset-types/*` | `admin` | Editor de tipos — precursor del Form Designer |
| `/propiedades/*` | `admin` | Definición y asociación de propiedades |

Las rutas operacionales (`/assets/*`, `/sensors/*`, `/events/*`, etc.) permanecen en
`assets`. Los maestros (`/regiones/*`, `/marcas/*`, etc.) se mueven a `masters`.

---

## 2. Estructura del Módulo

```
modules/
  admin/
    module.go           — registro del módulo, rutas, middleware
    handlers/
      users.go          — gestión de usuarios (extraído de internal/ui)
      settings.go       — configuración del sistema (extraído de internal/ui)
      permissions.go    — editor de permisos de formulario
      i18n.go           — gestor de paquetes de locale
      schemas.go        — ciclo de vida de schemas
      designer.go       — diseñador de formularios
      preview.go        — endpoint de preview del form engine
    pages/
      users.go          — páginas UI de gestión de usuarios
      settings.go       — páginas UI de configuración
      permissions.go    — páginas UI del editor de permisos
      i18n.go           — páginas UI del gestor de locales
      schemas.go        — páginas UI del ciclo de vida de schemas
      designer.go       — páginas UI del diseñador de formularios
    locales/
      admin.en.json     — claves i18n del módulo Admin en inglés
      admin.es.json     — claves i18n del módulo Admin en español
```

---

## 3. Sección 1 — Gestión de Usuarios

### 3.1 Estado actual

La gestión de usuarios está implementada en `internal/ui/admin_users.go` y
`internal/ui/routes.go`. El sistema de autenticación subyacente (RBAC, sesiones, CSRF,
brute-force, audit log, password reset) está completo según `docs/RBAC-COMPLETION-PLAN.md`.

Lo que existe:
- Listado de usuarios con tabs por rol (All / Admins / Managers / Operators / Viewers)
- Formulario de creación y edición de usuario
- Cambio de contraseña por admin
- Reset de contraseña self-service
- Cuatro roles: `admin`, `manager`, `operator`, `viewer`

### 3.2 Extensiones requeridas para el módulo Admin

**Roles adicionales:**

| Rol | Descripción |
|-----|-------------|
| `admin` | Acceso completo al sistema incluyendo módulo Admin |
| `seam_consultant` | Igual que `admin` más herramientas avanzadas del Form Designer |
| `manager` | Gestión operacional, sin acceso al módulo Admin |
| `operator` | Operación diaria, sin acceso al módulo Admin |
| `viewer` | Sólo lectura, sin acceso al módulo Admin |

El rol `seam_consultant` no puede crear ni modificar otros roles `seam_consultant` ni
`admin` — sólo puede gestionar usuarios de roles inferiores. Esto previene escalada de
privilegios por consultores externos.

**Campos adicionales en el modelo de usuario:**

```go
type User struct {
    // Campos existentes
    ID          int64
    Email       string
    Name        string
    Role        string
    Active      bool
    LastLoginAt time.Time
    CreatedAt   time.Time

    // Nuevos
    Locale      string    // locale preferido del usuario (override del tenant)
    TwoFactorEnabled bool // reservado — no implementado en MVP
    Notes       string    // nota interna del administrador sobre el usuario
    ExpiresAt   *time.Time // cuenta con expiración (ej: consultor temporal)
}
```

**Vistas adicionales:**

*Vista de detalle de usuario* — muestra toda la actividad del usuario: último acceso,
IP del último acceso, intentos de login fallidos recientes, registros del audit log
asociados a ese usuario. Útil para diagnóstico y auditoría.

*Vista de sesiones activas* — lista las sesiones activas del usuario con IP y timestamp.
Permite al admin invalidar sesiones individuales o todas las sesiones de un usuario.

### 3.3 Rutas del módulo Admin — Gestión de Usuarios

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/users` | `UsersList` | admin, seam_consultant |
| GET | `/admin/users/new` | `UserNew` | admin |
| POST | `/admin/users` | `UserCreate` | admin |
| GET | `/admin/users/{id}` | `UserDetail` | admin, seam_consultant |
| GET | `/admin/users/{id}/edit` | `UserEdit` | admin |
| POST | `/admin/users/{id}` | `UserUpdate` | admin |
| POST | `/admin/users/{id}/delete` | `UserDelete` | admin |
| GET | `/admin/users/{id}/password` | `UserPasswordPage` | admin |
| POST | `/admin/users/{id}/password` | `UserPasswordReset` | admin |
| POST | `/admin/users/{id}/sessions/invalidate` | `UserSessionsInvalidate` | admin |

---

## 4. Sección 2 — Configuración del Sistema

### 4.1 Estado actual

La configuración del sistema existe en `/settings` dentro del módulo principal. Cubre
configuración general (nombre del tenant, locale), notificaciones (email, Slack, webhook),
features (mapa de activos), y apariencia (duración de transiciones).

### 4.2 Estructura de configuración ampliada

La configuración del sistema se reorganiza en secciones claras dentro del módulo Admin:

#### General

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `tenant_name` | string | Nombre de la organización |
| `tenant_slug` | string | Identificador único del tenant (sólo lectura post-creación) |
| `default_locale` | string | Locale por defecto del tenant |
| `timezone` | string | Zona horaria (IANA, ej: `"America/Montevideo"`) |
| `date_format` | string | Formato de fecha preferido (puede diferir del locale) |
| `logo_url` | string | URL del logo del tenant para la cabecera |

#### Módulos activos

Lista de módulos compilados en la instancia actual. Para cada módulo: nombre, versión,
estado (activo / inactivo), y botón de activación/desactivación. Desactivar un módulo
oculta sus rutas y su entrada en el launcher pero no elimina sus datos.

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `modules.{nombre}.active` | boolean | Módulo visible para usuarios operacionales |
| `modules.{nombre}.roles` | []string | Roles que pueden acceder al módulo |

#### Seguridad

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `auth.session_duration` | string | Duración de sesión (ej: `"24h"`) |
| `auth.lockout_threshold` | int | Intentos antes de bloqueo |
| `auth.lockout_duration` | string | Duración del bloqueo |
| `auth.password_min_length` | int | Longitud mínima de contraseña |
| `auth.require_2fa` | boolean | Reservado — no implementado en MVP |

#### Notificaciones

| Canal | Campos |
|-------|--------|
| Email | `enabled`, `smtp_host`, `smtp_port`, `smtp_user`, `smtp_from` |
| Slack | `enabled`, `webhook_url`, `channel` |
| Webhook | `enabled`, `url`, `secret` (para firma HMAC), `retry_count` |

#### Features

Toggles de funcionalidades opcionales. Cada toggle tiene nombre, descripción, y estado
actual. Los toggles se declaran por módulo — el módulo Admin no conoce todos los posibles
toggles del sistema, sino que lee una lista registrada por cada módulo activo al iniciar.

#### Apariencia

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `ui.transition_duration` | string | Duración de transiciones CSS |
| `ui.default_theme` | string | `"light"` \| `"dark"` \| `"system"` |
| `ui.density` | string | `"comfortable"` \| `"compact"` — densidad visual de listados |

### 4.3 Rutas del módulo Admin — Configuración

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/settings` | `SettingsPage` | admin |
| POST | `/admin/settings/general` | `SettingsSaveGeneral` | admin |
| POST | `/admin/settings/modules` | `SettingsSaveModules` | admin |
| POST | `/admin/settings/security` | `SettingsSaveSecurity` | admin |
| POST | `/admin/settings/notifications` | `SettingsSaveNotifications` | admin |
| POST | `/admin/settings/features` | `SettingsSaveFeatures` | admin |
| POST | `/admin/settings/appearance` | `SettingsSaveAppearance` | admin |

---

## 5. Sección 3 — Gestor de Permisos

### 5.1 Propósito

El Gestor de Permisos es la UI para administrar el modelo `FieldPermission` definido en
`docs/FORM_ENGINE_SPEC.md` §3c. Permite a un administrador definir qué elementos de qué
formularios son visibles, de sólo lectura, u ocultos para cada rol.

### 5.2 Vistas principales

#### Vista de formulario

Punto de entrada principal. Muestra todos los formularios del sistema agrupados por módulo.
Para cada formulario, un indicador visual muestra si tiene restricciones de permisos activas
(ninguna / algunas / muchas).

Al seleccionar un formulario, se despliega el árbol completo de su estructura:
tabs → secciones → campos, más los paneles `detail` y sub-formularios `embed`. Cada
elemento del árbol muestra el nivel de acceso para cada rol en una fila horizontal de
badges coloreados:

```
                    admin    manager  operator  viewer
──────────────────────────────────────────────────────
Tab: Principal       ●edit    ●edit    ●edit    ●read
  Sección: Básico    ●edit    ●edit    ●edit    ●read
    Campo: nombre    ●edit    ●edit    ●edit    ●read
    Campo: tipo_id   ●edit    ●edit    ●read    ●hide
  Sección: Finanzas  ●edit    ●edit    ●hide    ●hide
    Campo: precio    ──       ──       ──       ──    (heredado)
Tab: Finanzas        ●edit    ●edit    ●hide    ●hide
```

Los campos con `──` tienen acceso heredado de la sección o tab padre — no tienen regla
propia. Esto distingue claramente entre reglas directas e heredadas.

Clicking en cualquier badge abre un editor inline que permite cambiar el acceso para ese
rol en ese elemento.

#### Vista de rol

Vista inversa: dado un rol, muestra todo lo que ese rol puede ver, editar u ocultar a
través de todos los formularios del sistema. Útil para auditoría de permisos antes de
asignar un rol a un usuario nuevo.

Organizado por módulo → formulario → elemento. Muestra el nivel de acceso efectivo
(considerando herencia) y si viene de una regla directa o heredada.

#### Vista de diferencias

Compara el nivel de acceso de dos roles lado a lado para un formulario dado. Útil para
verificar que el perfil de permisos de un rol nuevo es correcto respecto a uno existente.

### 5.3 Herencia de permisos

Una regla sobre un tab tiene precedencia sobre las reglas de sus secciones y campos.
Una regla sobre una sección tiene precedencia sobre las reglas de sus campos.
El administrador puede sobreescribir la herencia definiendo una regla directa en el
elemento hijo.

El editor muestra para cada elemento:
- Regla directa (si existe): badge con fondo sólido
- Regla heredada (si no hay directa): badge con borde y fondo translúcido
- Sin regla ni herencia: badge gris "default (editable)"

Al editar una regla heredada, el editor pregunta: "¿Crear regla directa para este
elemento, o modificar la regla del padre?"

### 5.4 Resolución de conflictos multi-rol

La resolución de conflictos cuando un usuario tiene múltiples roles sigue la **opción C**
de Risk §15.3: prioridad explícita de rol. Los roles tienen un orden de prioridad declarado
en la configuración del sistema:

```json
{
  "roles": [
    { "name": "admin",            "priority": 100 },
    { "name": "seam_consultant", "priority": 90  },
    { "name": "manager",          "priority": 50  },
    { "name": "operator",         "priority": 30  },
    { "name": "viewer",           "priority": 10  }
  ]
}
```

Cuando un usuario tiene múltiples roles con reglas contradictorias, gana la regla del
rol de mayor prioridad. El Gestor de Permisos muestra esta resolución explícitamente
cuando hay conflictos potenciales.

### 5.5 Permisos de permisos

El Gestor de Permisos es accesible sólo para `admin` y `seam_consultant`. Sin embargo,
el rol `seam_consultant` no puede:

- Modificar permisos del rol `admin`
- Elevar sus propios permisos o los de otro `seam_consultant`
- Crear reglas que den acceso a elementos marcados como `system_only` en el schema

### 5.6 Rutas del módulo Admin — Gestor de Permisos

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/permissions` | `PermissionsIndex` | admin, seam_consultant |
| GET | `/admin/permissions/{module}/{form}` | `PermissionsForm` | admin, seam_consultant |
| GET | `/admin/permissions/role/{role}` | `PermissionsRole` | admin, seam_consultant |
| GET | `/admin/permissions/diff/{role_a}/{role_b}` | `PermissionsDiff` | admin, seam_consultant |
| POST | `/admin/permissions/{module}/{form}/{element}` | `PermissionSave` | admin, seam_consultant |
| DELETE | `/admin/permissions/{module}/{form}/{element}/{role}` | `PermissionDelete` | admin, seam_consultant |

---

## 6. Sección 4 — Gestor de Internacionalización

### 6.1 Propósito

El Gestor de i18n es la UI para administrar los paquetes de locale definidos en
`docs/I18N_SPEC.md`. Permite a un administrador ver, editar, e importar/exportar
traducciones sin editar archivos JSON directamente.

### 6.2 Vistas principales

#### Vista de locales

Listado de todos los paquetes de locale instalados. Para cada locale:

| Columna | Contenido |
|---------|-----------|
| Código | BCP 47 (ej: `es`, `ar`) |
| Nombre | Nombre nativo del idioma |
| Dirección | LTR / RTL con indicador visual |
| Completitud | Barra de progreso: claves traducidas / total de claves del locale base `en` |
| Claves de schema | Claves derivadas de schemas en xolu no traducidas (número en badge naranja) |
| Estado | Activo / Inactivo |
| Acciones | Editar, Exportar, Activar/Desactivar |

El botón "Nuevo Locale" permite crear un paquete nuevo a partir de una plantilla vacía
o copiando un locale existente como punto de partida.

#### Vista de edición de locale

Selector de módulo en la parte superior. Al elegir un módulo, se muestra la lista de
secciones del módulo en un panel lateral izquierdo. Al elegir una sección, se muestra el
editor de dos columnas:

```
┌─────────────────┬──────────────────────────────────────────────────────┐
│  Módulos        │  Sección: assets › forms › asset › field             │
│  ─────────────  │  ─────────────────────────────────────────────────── │
│  core           │  Clave                    EN (ref)      ES (editar)  │
│  ▶ assets       │  nombre                   Name          Nombre       │
│    forms        │  tipo_id                  Asset Type    Tipo de...   │
│      asset ●    │  status                   Status        Estado       │
│      sensor     │  criticidad               Criticality   ✗ PENDIENTE  │
│      asset_type │  connectivity_tier        Connectivity  Conectiv...  │
│  masters        │                                                       │
│  admin          │  [ Guardar sección ]                                  │
└─────────────────┴──────────────────────────────────────────────────────┘
```

Las claves pendientes (presentes en `en` pero ausentes en el locale editado) se destacan
con una etiqueta `✗ PENDIENTE` y fondo amarillo suave. El campo de edición está vacío
y con placeholder mostrando el texto en inglés.

El módulo con el indicador `●` tiene claves pendientes. La cuenta de pendientes aparece
junto al nombre del módulo en el panel lateral.

#### Vista de claves de schema

Sección especial que muestra las claves derivadas de los schemas en xolu que no existen
en ningún paquete de locale. Estas son las claves más urgentes de traducir porque son
directamente visibles en la UI operacional.

Organizado por módulo → formulario → elemento (tab / sección / campo / detail / embed).
Permite añadir la traducción inline sin salir de esta vista.

#### Vista de vocabulario del tenant

Editor del namespace `tenant.terms.*`. Permite al administrador definir terminología
propia:

| Término estándar | Término del tenant | Plural |
|-----------------|-------------------|--------|
| asset | equipo | equipos |
| sensor | dispositivo | dispositivos |
| custodio | responsable | responsables |

Estos términos se aplican en toda la UI cuando el locale del tenant está activo.

### 6.3 Import / Export

**Exportar:** genera un archivo JSON del paquete completo del locale seleccionado.
El archivo incluye todos los módulos y su estado actual (traducido / pendiente).
Las claves pendientes se incluyen con valor `null` para facilitar la búsqueda en un
editor externo.

**Importar:** acepta un archivo JSON del mismo formato. Valida la estructura antes de
aplicar. Muestra un diff de cambios (claves nuevas, claves modificadas, claves eliminadas)
y pide confirmación antes de persistir.

**Importación parcial:** permite importar sólo un módulo o sección específica del archivo,
útil cuando el trabajo de traducción se divide entre varias personas.

### 6.4 Rutas del módulo Admin — Gestor de i18n

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/i18n` | `I18nIndex` | admin, seam_consultant |
| GET | `/admin/i18n/{locale}` | `I18nLocale` | admin, seam_consultant |
| GET | `/admin/i18n/{locale}/{module}` | `I18nModule` | admin, seam_consultant |
| GET | `/admin/i18n/{locale}/{module}/{section}` | `I18nSection` | admin, seam_consultant |
| POST | `/admin/i18n/{locale}/{module}/{section}` | `I18nSectionSave` | admin, seam_consultant |
| GET | `/admin/i18n/{locale}/schemas` | `I18nSchemaKeys` | admin, seam_consultant |
| POST | `/admin/i18n/{locale}/schemas/{key}` | `I18nSchemaKeySave` | admin, seam_consultant |
| GET | `/admin/i18n/{locale}/terms` | `I18nTerms` | admin |
| POST | `/admin/i18n/{locale}/terms` | `I18nTermsSave` | admin |
| GET | `/admin/i18n/{locale}/export` | `I18nExport` | admin, seam_consultant |
| POST | `/admin/i18n/{locale}/import` | `I18nImport` | admin |
| POST | `/admin/i18n/{locale}/activate` | `I18nActivate` | admin |
| POST | `/admin/i18n` | `I18nCreate` | admin |

---

## 7. Sección 5 — Ciclo de Vida de Schemas

### 7.1 Propósito

El Ciclo de Vida de Schemas gestiona el flujo desde la creación de un schema de formulario
hasta su publicación en producción, incluyendo versionado, compatibilidad, y rollback.
Implementa la política definida en Risk §15.5 del Form Engine Spec.

### 7.2 Estados de un schema

```
draft ──── (validar) ──── compatible ──── (publicar) ──── published
  │                           │                               │
  │                    semi-compatible                   (archivar)
  │                           │                               │
  └──────────────────── breaking                          archived
                              │
                        (migración requerida)
```

| Estado | Descripción |
|--------|-------------|
| `draft` | Schema en edición. No afecta a los usuarios. Puede coexistir con un schema publicado. |
| `published` | Schema activo. El form engine lo usa para renderizar. Sólo puede haber uno publicado por formulario. |
| `archived` | Schema retirado. Los registros creados con este schema se muestran en modo read-only con él. |

### 7.3 Modelo `SchemaVersion`

```go
type SchemaVersion struct {
    ID          string    // UUID
    TenantID    string
    Module      string    // "assets"
    Form        string    // "asset"
    Version     int       // incrementado en cada publicación
    Schema      []byte    // JSON Schema completo con extensiones x-seam-*
    Status      string    // "draft" | "published" | "archived"
    CreatedBy   string    // user ID
    PublishedBy string    // user ID (vacío si draft)
    PublishedAt *time.Time
    ArchivedAt  *time.Time
    Notes       string    // notas del editor sobre este draft
    Compat      string    // "additive" | "semi" | "breaking" (calculado al publicar)
}
```

### 7.4 Verificación de compatibilidad

Al intentar publicar un draft sobre un schema publicado existente, el sistema ejecuta
una verificación de compatibilidad comparando ambas versiones:

**Additive (compatible):** sólo se añaden campos opcionales, secciones, o tabs.
Ningún campo existente es eliminado ni se le añade obligatoriedad. Los registros existentes
no requieren migración. Publicación inmediata.

**Semi-compatible:** se añaden campos obligatorios, o se cambia la obligatoriedad de
campos opcionales a requeridos. Los registros existentes pueden leerse pero no guardarse
hasta que los nuevos campos requeridos sean completados. El sistema muestra cuántos
registros se ven afectados y permite continuar con advertencia.

**Breaking:** se eliminan campos, se cambian tipos, o se renombran campos existentes.
Los registros creados con la versión anterior quedan vinculados a ella y se muestran en
modo read-only. El administrador debe aprobar explícitamente un breaking change y
opcionalmente iniciar un proceso de migración asistida.

### 7.5 Vista de schemas

#### Listado de schemas

Tabla de todos los formularios del sistema con su schema actual. Para cada formulario:

| Columna | Contenido |
|---------|-----------|
| Formulario | `{módulo} › {formulario}` |
| Versión publicada | Número de versión y fecha de publicación |
| Draft activo | Indicador si hay un draft en edición |
| Última modificación | Quién y cuándo |
| Acciones | Ver, Nuevo draft, Historial |

#### Vista de historial de un formulario

Línea de tiempo con todas las versiones del schema de un formulario. Para cada versión:
número, estado, fecha, autor, tipo de compatibilidad (si publicada), y botón para ver el
schema completo o restaurar como draft.

#### Vista de draft

Editor JSON del schema con resaltado de sintaxis. Para el rol `seam_consultant`, el
editor JSON es directamente editable. Para el rol `admin` en modo DIY, el JSON se muestra
como referencia pero la edición se realiza desde el Form Designer (§8).

Panel lateral de validación en tiempo real: mientras el JSON se edita, el servidor valida
el schema via queryfy y muestra errores y advertencias.

Panel de comparación: diff visual entre el draft y el schema publicado actual, resaltando
añadidos (verde), eliminados (rojo), y modificados (amarillo).

### 7.6 Rutas del módulo Admin — Ciclo de Vida de Schemas

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/schemas` | `SchemasIndex` | admin, seam_consultant |
| GET | `/admin/schemas/{module}/{form}` | `SchemaHistory` | admin, seam_consultant |
| GET | `/admin/schemas/{module}/{form}/draft` | `SchemaDraft` | admin, seam_consultant |
| POST | `/admin/schemas/{module}/{form}/draft` | `SchemaDraftSave` | admin, seam_consultant |
| POST | `/admin/schemas/{module}/{form}/draft/validate` | `SchemaDraftValidate` | admin, seam_consultant |
| POST | `/admin/schemas/{module}/{form}/publish` | `SchemaPublish` | admin |
| POST | `/admin/schemas/{module}/{form}/{version}/restore` | `SchemaRestore` | admin |
| GET | `/admin/schemas/{module}/{form}/{version}` | `SchemaVersionView` | admin, seam_consultant |

---

## 8. Sección 6 — Diseñador de Formularios

### 8.1 Propósito

El Diseñador de Formularios es el componente más complejo del módulo Admin. Es una
herramienta visual donde el administrador construye o modifica la estructura de un
formulario mediante bloques jerárquicos, viendo el resultado renderizado en tiempo real.

El Diseñador produce un JSON Schema con extensiones `x-seam-*` válido y lo persiste como
draft en el Ciclo de Vida de Schemas. La publicación del draft activa el nuevo formulario
en producción.

### 8.2 Arquitectura de la UI

La UI del Diseñador es una pantalla de ancho completo dividida en tres paneles:

```
┌──────────────────┬───────────────────────────────┬──────────────────┐
│  ÁRBOL DE        │  CONFIGURADOR                 │  PREVIEW         │
│  BLOQUES         │                               │                  │
│                  │  [elemento seleccionado]       │  [form engine    │
│  Formulario      │                               │   output]        │
│  ├─ Tab 1        │  Propiedades del bloque        │                  │
│  │  ├─ Fila 1    │  seleccionado en el árbol      │  [locale         │
│  │  │  ├─ Sec A  │                               │   selector]      │
│  │  │  └─ Sec B  │  ┌─────────────────────┐     │                  │
│  │  └─ Fila 2    │  │  label              │     │  [role           │
│  │     └─ Sec C  │  │  layout             │     │   selector]      │
│  ├─ Tab 2        │  │  visible_when       │     │                  │
│  └─ Tab 3        │  │  ...                │     │                  │
│                  │  └─────────────────────┘     │                  │
│  [+ Añadir]      │                               │                  │
└──────────────────┴───────────────────────────────┴──────────────────┘
```

**Panel izquierdo — Árbol de bloques:** representación jerárquica del schema como árbol
de bloques colapsables. Cada nodo es un elemento del formulario (formulario, tab, fila,
sección, campo, detail, embed). Los bloques pueden:
- Seleccionarse (clic) para editar sus propiedades en el panel central
- Reordenarse (drag and drop dentro de su nivel)
- Añadirse (botón `+` contextual en cada nivel)
- Eliminarse (botón `×` al hacer hover)
- Copiarse (duplicar un bloque y sus hijos)

**Panel central — Configurador:** formulario de propiedades del bloque seleccionado.
Las propiedades mostradas dependen del tipo de bloque (ver §8.3). Actualiza el preview
al guardar cada cambio.

**Panel derecho — Preview:** renderizado en tiempo real del formulario usando el form
engine del servidor. Se actualiza vía htmx cada vez que el configurador guarda un cambio
en el draft. Muestra el formulario exactamente como lo verá el usuario final.

El selector de locale en el panel de preview permite ver el formulario en cualquier locale
instalado, revelando claves sin traducción con un indicador visual `[?]` sobre el label.

El selector de rol en el panel de preview simula la vista del formulario desde la
perspectiva de un rol específico, aplicando las reglas de `FieldPermission` vigentes.

### 8.3 Configurador por tipo de bloque

#### Formulario (raíz)

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `width` | Select | sm / md / lg / xl / 2xl / fullscreen |
| `max_height` | Input text | Altura máxima del contenedor |
| Módulo | Select (readonly) | Módulo al que pertenece este formulario |
| Nombre | Input text | Identificador del formulario en el namespace i18n |

#### Tab

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `id` | Input text | Identificador único |
| `label` | Clave i18n | Selector de clave + preview del texto traducido |
| `visible_when` | Editor de condición | Constructor visual de condiciones (ver §8.5) |

#### Fila

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `widths` | Selector proporcional | Visualización de 12 columnas con sliders por sección |
| `visible_when` | Editor de condición | Constructor visual de condiciones |

#### Sección

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `id` | Input text | Identificador único |
| `label` | Clave i18n | Selector de clave + preview |
| `layout` | Select | 1-col / 2-col |
| `visible_when` | Editor de condición | Constructor visual de condiciones |

#### Campo

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `name` | Input text | Nombre del campo (clave en `properties`) |
| `title` | Clave i18n | Label del campo |
| `type` | Select | string / integer / number / boolean / array |
| `format` | Select (contextual) | email / uri / date / date-time / time / tel |
| `x-seam-input` | Select | Todos los widgets disponibles (ver tabla §14 del Form Engine Spec) |
| `x-seam-source` | Select | Entidades del sistema disponibles |
| `x-seam-fk-popover` | Input text / toggle | URL base para popover |
| `x-seam-help` | Textarea | Texto de ayuda (multilínea) |
| `x-seam-help-style` | Select | inline / tooltip |
| `x-seam-rows` | Input number | Filas para textarea |
| `x-seam-relation` | Select | contiene / usa (para asset-picker) |
| `required` | Toggle | Campo obligatorio |
| `minLength` / `maxLength` | Input number | Restricciones de longitud |
| `minimum` / `maximum` | Input number | Restricciones de valor |
| `pattern` | Input text | Patrón de validación regex |
| `enum` | Editor de lista | Lista de valores permitidos (agregar/eliminar) |
| `x-seam-visible-when` | Editor de condición | Constructor visual de condición |

#### Panel `detail`

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `id` | Input text | Identificador único |
| `label` | Clave i18n | Título del panel |
| `source` | Input text | URL del endpoint (con `{id}` si aplica) |
| `columns` | Editor de lista | Claves de columna con sus labels i18n |
| `empty` | Clave i18n | Mensaje de estado vacío |
| `position` | Select | bottom / right / inline |
| `collapsible` | Toggle | Permite colapsar |
| `load` | Select | eager / lazy |

#### Sub-formulario `embed`

| Propiedad | Control | Descripción |
|-----------|---------|-------------|
| `id` | Input text | Identificador único |
| `label` | Clave i18n | Título del embed |
| `endpoint` | Input text | URL del endpoint de acción |
| `method` | Select | post / put / delete |
| `trigger` | Select | button / inline / auto |
| `trigger_label` | Clave i18n | Label del botón disparador |
| `mode` | Select | modal2 / inline / drawer |
| `schema` | Select | Schema del sub-formulario (de la lista de schemas en xolu) |
| `confirm` | Clave i18n | Mensaje de confirmación (opcional) |
| `refresh_detail` | Select | Panel detail a recargar tras submit |

### 8.4 Selector de clave i18n

Cuando una propiedad requiere una clave i18n (labels de tabs, secciones, campos, etc.),
el control muestra:

1. Un campo de búsqueda que filtra las claves existentes del namespace del formulario
2. Una preview del texto traducido en el locale activo del editor
3. Si la clave no existe: opción "Crear clave nueva" que abre un mini-editor para añadir
   la traducción en todos los locales instalados simultáneamente
4. Para claves de campos: el nombre del campo es el mismo que la clave i18n por convención
   (`field.{nombre}`), así el administrador no necesita gestionar claves manualmente en
   casos simples

### 8.5 Editor visual de condiciones `visible_when`

El constructor visual de condiciones reemplaza la escritura manual del JSON de
`visible_when`. Presenta una UI del estilo "regla de filtro":

```
┌────────────────────────────────────────────────────────────┐
│  Mostrar cuando:                                           │
│                                                            │
│  [calidad_tenencia ▼]  [es igual a ▼]  [Comodato ▼]  [×] │
│                                                            │
│  [+ Añadir condición]     ○ Todas  ○ Cualquiera           │
└────────────────────────────────────────────────────────────┘
```

- El primer selector muestra los campos del formulario con sus labels i18n
- El segundo selector muestra los operadores disponibles para el tipo del campo elegido
- El tercer control es un input o select según el tipo del campo
- El toggle "Todas / Cualquiera" corresponde a `all_of` / `any_of`
- Los grupos pueden anidarse con el botón "Añadir grupo"

El JSON resultante se muestra en un desplegable "Ver JSON" para el rol `seam_consultant`.

### 8.6 Endpoint de preview

El preview se genera llamando al endpoint `/admin/designer/preview` con el draft schema
actual, el locale seleccionado, el rol seleccionado, y el ID de entidad (si es modo
edición). El servidor:

1. Deserializa el draft schema
2. Resuelve los permisos del rol seleccionado
3. Llama al form engine con un `RenderContext` construido a partir de esos parámetros
4. Retorna el HTML renderizado

El resultado reemplaza el panel de preview via htmx. La latencia objetivo es < 150ms
para que la experiencia se sienta en tiempo real.

Para el preview con datos reales (modo edición), el endpoint acepta un `entity_id`
opcional. Si se provee, carga los valores actuales del registro para pre-popular el
formulario en el preview. Esto permite al diseñador verificar que el formulario se ve
correctamente con datos reales.

### 8.7 Modos del Diseñador

#### Modo guiado (DIY — rol `admin`)

- Panel de árbol con bloques pre-configurados disponibles para añadir
- Configurador con controles visuales para todas las propiedades
- Sin acceso directo al JSON del schema
- Validación inmediata de cada campo del configurador antes de cerrar
- Mensajes de error en lenguaje no técnico

#### Modo avanzado (consultor — rol `seam_consultant`)

Todo lo del modo guiado más:

- Panel de código JSON editable directamente, sincronizado con el árbol visual
- Sincronización bidireccional: editar el árbol actualiza el JSON, editar el JSON
  actualiza el árbol
- Herramientas de diagnóstico: inspector de schema compilado, log de errores de
  compilación, diff detallado con el schema publicado
- Importación de schema desde archivo JSON externo
- Aplicación de cambios en lote (ej: renombrar un campo en todos sus contextos)

### 8.8 Gestión de claves i18n desde el Diseñador

Cuando el diseñador añade un elemento nuevo (campo, sección, tab) que introduce claves
i18n no existentes, el Diseñador muestra un indicador inmediato en el panel de árbol y
en el panel de preview. Al intentar publicar el draft con claves pendientes, el sistema:

1. Muestra la lista de claves faltantes por locale instalado
2. Permite resolver las traducciones inline antes de publicar
3. O permite publicar con claves faltantes (que aparecerán como texto inglés para usuarios
   de otros locales) con una advertencia explícita

### 8.9 Rutas del módulo Admin — Diseñador de Formularios

| Método | Ruta | Handler | Acceso |
|--------|------|---------|--------|
| GET | `/admin/designer` | `DesignerIndex` | admin, seam_consultant |
| GET | `/admin/designer/{module}/{form}` | `DesignerForm` | admin, seam_consultant |
| POST | `/admin/designer/{module}/{form}/block` | `DesignerBlockAdd` | admin, seam_consultant |
| PUT | `/admin/designer/{module}/{form}/block/{id}` | `DesignerBlockUpdate` | admin, seam_consultant |
| DELETE | `/admin/designer/{module}/{form}/block/{id}` | `DesignerBlockDelete` | admin, seam_consultant |
| POST | `/admin/designer/{module}/{form}/reorder` | `DesignerReorder` | admin, seam_consultant |
| POST | `/admin/designer/preview` | `DesignerPreview` | admin, seam_consultant |
| POST | `/admin/designer/{module}/{form}/i18n` | `DesignerI18nSave` | admin, seam_consultant |

---

## 9. Navegación del Módulo Admin

El módulo Admin tiene su propio layout de navegación, distinto del layout operacional.
La navegación del módulo Admin es un panel lateral izquierdo con las secciones en el
orden de dependencia:

```
SEAM ADMIN
──────────────────
  Usuarios
  Configuración
  Permisos
  Idiomas
  Schemas
  Diseñador
──────────────────
  ← Volver a [módulo operacional]
```

El enlace "Volver" lleva al último módulo operacional visitado, o a `assets` por defecto.

En el launcher principal (equivalente al Google Workspace launcher), el módulo Admin
aparece como una aplicación separada, accesible sólo para roles `admin` y
`seam_consultant`. El ícono y el nombre son configurables en las preferencias de
apariencia del sistema.

---

## 10. Registro de Auditoría del Módulo Admin

Todas las acciones administrativas se registran en el audit log existente. Las categorías
adicionales específicas del módulo Admin:

| Categoría | Eventos |
|-----------|---------|
| `admin.schema` | `schema.draft.created`, `schema.draft.saved`, `schema.published`, `schema.rolled_back` |
| `admin.permission` | `permission.created`, `permission.updated`, `permission.deleted` |
| `admin.i18n` | `locale.activated`, `locale.deactivated`, `locale.section.saved`, `locale.imported` |
| `admin.settings` | `settings.{section}.saved` |
| `admin.designer` | `designer.block.added`, `designer.block.updated`, `designer.block.deleted`, `designer.reordered` |

El audit log del módulo Admin es accesible desde la vista de detalle de usuario (para
ver acciones de un usuario específico) y desde una vista global en el módulo Admin
(para ver todas las acciones administrativas cronológicamente).

---

## 11. Riesgos Conocidos y Decisiones Pendientes

### 11.1 Sincronización bidireccional árbol ↔ JSON en modo avanzado

**Riesgo:** La sincronización bidireccional entre el árbol visual y el editor JSON en
modo `seam_consultant` es técnicamente compleja. Si el JSON editado directamente
contiene construcciones válidas para queryfy pero no representables en el árbol visual
(ej: condiciones `if/then` complejas, propiedades no reconocidas por el Diseñador), el
árbol no puede representarlas y la sincronización falla silenciosamente o destruye
información.

**Mitigación recomendada:** La sincronización es unidireccional en caso de ambigüedad:
el JSON es siempre la fuente de verdad. El árbol es una vista derivada del JSON. Si el
JSON contiene construcciones que el árbol no puede representar, esas construcciones se
muestran en el árbol como un nodo opaco "Bloque avanzado" no editable visualmente pero
preservado intacto en el JSON.

**Decisión pendiente:** definir el conjunto exacto de construcciones "avanzadas" que el
árbol no intenta representar, antes de implementar el parser árbol→JSON.

### 11.2 Latencia del endpoint de preview

**Riesgo:** El objetivo de < 150ms para el preview puede ser difícil de cumplir si el
formulario es complejo (muchos campos, múltiples sources FK que requieren consultas a xolu)
o si el servidor está bajo carga. Una experiencia de preview lenta es percibida como una
herramienta rota por el usuario DIY.

**Mitigaciones:**
- Pre-cargar todas las fuentes FK al abrir el Diseñador y cachearlas en la sesión del admin
- El preview usa datos cacheados, no consultas en tiempo real a xolu
- El endpoint de preview tiene su propio timeout corto (500ms) con fallback a un mensaje
  de "Preview no disponible temporalmente" en lugar de bloquear la UI

**Decisión pendiente:** definir la estrategia de cache de fuentes FK antes de implementar
el endpoint de preview.

### 11.3 Conflicto de edición concurrente del mismo schema

**Riesgo:** Si dos administradores editan el draft del mismo formulario simultáneamente,
el último en guardar sobreescribe los cambios del primero sin advertencia.

**Mitigación mínima:** Al abrir un draft para edición, el sistema registra quién lo tiene
abierto (lock optimista basado en timestamp). Si otro usuario intenta abrir el mismo draft,
recibe un aviso: "Este schema está siendo editado por {nombre} desde {tiempo}. ¿Abrir
de sólo lectura o tomar el control?"

Tomar el control invalida la sesión de edición del otro usuario y le muestra un aviso
en su Diseñador.

**Decisión pendiente:** confirmar si el lock optimista es suficiente o se requiere lock
pesimista (bloqueo exclusivo) para el contexto de uso esperado.

### 11.4 Impacto de un schema roto en producción

**Riesgo:** Si se publica un schema con un error no detectado por la validación (ej: una
clave i18n que existe en el schema pero no en ningún locale, o un `x-seam-source` que
apunta a una entidad no existente), el formulario en producción puede renderizarse
incorrectamente o fallar.

**Mitigación:** El sistema ejecuta un "dry run" del form engine completo sobre el draft
antes de permitir la publicación. El dry run renderiza el formulario con datos sintéticos
para todos los roles y todos los locales instalados, capturando cualquier error de
renderizado antes de que llegue a producción. Si el dry run falla para algún rol o locale,
la publicación se bloquea y se muestra el error específico.

**Decisión pendiente:** definir qué datos sintéticos usa el dry run para los campos FK
(IDs ficticios, o los primeros registros reales de cada entidad en el tenant de prueba).

## Referencia cruzada: módulo Developer

Updated: 2026-11-17

El módulo Developer (`docs/DEVELOPER_MODULE_CAMPAIGN.md`) contiene el diseñador de formularios de la sección 8 (campos y árbol de bloques: paleta, inspector, pestañas, secciones, filas, arrastrar y soltar, duplicado, vista previa en vivo en cualquier idioma y guardado en cualquier alcance de override; DEV-21, DEC-29, DEC-37, DEC-39) y las piezas que necesita el ciclo de vida de esquemas: capas de override, compuerta de validación, notas de compatibilidad e historial de versiones con restauración (DEV-14 a DEV-17, DEC-32 a DEC-34). El árbol de bloques (pestañas, filas, secciones) ya está en ese diseñador; los detalles y embeds se editan allí como listas JSON. El trabajo de esta sección debería partir de él.

---

*Documento producido en sesión de diseño de arquitectura de Seam AMS, Mayo 2026.*  
*Copyright (c) 2026 haitch — h@ual.li*
