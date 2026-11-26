# Seam AMS — Catálogo de Datos de Demostración

**Actualizado:** noviembre de 2026
**Documento interno — Seam S.A.**

---

## 1. Introducción

Seam AMS incluye siete conjuntos de datos de demostración (*seeds*) que
permiten poblar una instancia vacía de xolu con datos realistas. Los cuatro
primeros representan cada uno una empresa ficticia en un sector industrial
diferente, con activos, sensores, reglas, alertas y eventos coherentes; los
seeds 05 y 06 son el seed 01 con todo su contenido en inglés y en japonés
(sección 2), y el seed 07 es un grupo hotelero con su contenido en portugués
(sección 6).

Los seeds cumplen tres funciones:

- **Demostración comercial:** permiten mostrar la plataforma funcionando
  con datos realistas sin necesidad de un cliente real.
- **Validación del modelo de datos:** ejercitan los distintos tipos de
  activo, propiedades dinámicas, FSMs, reglas y eventos en combinaciones
  que revelan problemas de diseño.
- **Prueba de verticales:** cada seed prueba un sector industrial distinto,
  validando que la arquitectura genérica de Seam AMS se adapta a dominios
  con requisitos muy diferentes.

Cada seed es un archivo `.seed`: texto con un registro JSON por entidad,
en el formato que describe `docs/SEED_FORMAT.md`. Todos tienen las mismas
20 entidades y se cargan con el mismo comando:

```
python3 seeds/seed-XX-nombre.py [--url http://localhost:9090] [--dry-run]
```

Cada seed carga una sola empresa, en el tenant 1, sobre una instancia vacía:
no se combinan entre sí. Las cifras de este catálogo son las de lo que cada
seed crea, contadas sobre su modo de prueba (`--dry-run`). Además de los siete
seeds hay tres complementos opcionales (sección 10): los seeds de órdenes de
trabajo, los esquemas de entidades del seed 01 y el documento de dominio del
seed 04.

Las empresas ficticias fueron verificadas contra registros comerciales
reales para evitar coincidencias con nombres de empresas existentes en
los mercados objetivo.

---

## 2. Seed 01 — Constructec S.A.

**Archivo:** `seeds/seed-01-constructec.seed` (se carga con `seeds/seed-01-constructec.py`)
**Vertical:** Gestión de maquinaria e instalaciones de construcción
**Mercado:** Uruguay — regiones Metropolitana, Norte, Sur, Este y Centro
**Empresa:** Constructec S.A., empresa uruguaya de gestión de instalaciones
y equipos de construcción.

### Perfil de la empresa

Constructec opera maquinaria de construcción y agrícola, vehículos de
flota y grupos electrógenos distribuidos en obras, talleres y depósitos,
y además el equipamiento de sus propias instalaciones: climatización,
servidores y UPS de su centro de datos. La empresa necesita saber dónde
está cada equipo, quién lo tiene asignado, cuántas horas o kilómetros
acumula y cuándo corresponde el próximo servicio de mantenimiento.

### Características del seed

| Dimensión | Valor |
|---|---|
| Activos | 16 |
| Tipos de activo | 7 (maquinaria pesada, vehículo liviano, grupo electrógeno, climatización, servidor, UPS, maquinaria agrícola) |
| Marcas | 8 (Caterpillar, Toyota, Carrier, Dell, APC, Cisco, John Deere, Lenovo) |
| Modelos | 15 |
| Ubicaciones | 7 (sede, centro de datos, taller, planta, depósito, campo, obra) |
| Custodios | 5 |
| Sensores | 10 (4 GPS/telemática, 2 temperatura y humedad, 2 monitores de energía, 2 controladores de generador) |
| Reglas | 5 (kilómetros de vehículo, horas de maquinaria, fuera de zona geográfica, sin reporte GPS, generador sin arranque de prueba) |
| FSM | 2 (ciclo de vida de vehículo, ciclo de vida de maquinaria pesada) |
| Moneda | UYU, USD, EUR |

### Qué valida este seed

- Activos de alto valor unitario (USD 50K–500K)
- Seguimiento GPS en tiempo real
- Contadores de horas de motor y de kilómetros como driver de mantenimiento
- Activos de instalación fija (servidores, UPS, climatización) junto a la flota
- Asignación de custodios (operadores de obra)
- Múltiples regiones geográficas dentro de un país

### Variantes en otros idiomas: seeds 05 y 06

**Archivos:** `seeds/seed-05-fairborne.seed`, `seeds/seed-06-asahikenki.seed`

Son el mismo conjunto de datos que el seed 01 (16 activos, 10 sensores,
5 reglas, 2 FSMs), con todo el texto visible traducido:

| Seed | Empresa | Idioma del contenido | Para qué sirve |
|---|---|---|---|
| 05 — Fairborne | Fairborne Equipment LLC, noroeste del Pacífico | Inglés | Demostraciones en inglés |
| 06 — Asahi Kenki | Asahi Kenki K.K., región de Kanto | Japonés | Probar la interfaz con una escritura no latina |

Los campos restringidos a una lista de valores conservan los tokens que xolu
exige hoy (`criticidad`, `calidad_tenencia`, `subject_type` y otros): no son
texto libre y traducirlos rompería la validación.

---

## 3. Seed 02 — FrigoCold S.A.

**Archivo:** `seeds/seed-02-frigocold.seed` (se carga con `seeds/seed-02-frigocold.py`)
**Vertical:** Cadena de frío — refrigeración comercial e industrial
**Mercado:** Uruguay — distribución y logística refrigerada
**Empresa:** FrigoCold S.A., empresa uruguaya de instalación y monitoreo
de equipos de cadena de frío.

### Perfil de la empresa

FrigoCold instala y monitorea cámaras frigoríficas de conservación y
congelación, unidades de transporte refrigerado, y equipos de frío
farmacéutico en clientes del sector alimentario, farmacéutico y logístico.
La empresa es responsable de que la cadena de frío no se rompa.

### Características del seed

| Dimensión | Valor |
|---|---|
| Activos | 18 |
| Tipos de activo | 7 (cámara de conservación, cámara de congelación, vitrina vertical, vitrina horizontal, rack de compresores, frío farmacéutico, transporte refrigerado) |
| Marcas | 10 (Carrier, Hussmann, Arneg, Bohn/Heatcraft, Bitzer, Tecumseh, Danfoss, Embraco, Trane, Larquier) |
| Modelos | 18 |
| Ubicaciones | 10 (taller y depósito propios, 7 sitios de clientes, base de transporte) |
| Custodios | 6 (FrigoCold + 5) |
| Sensores | 20 (13 sondas de temperatura, 2 contactos de puerta, 3 transductores de presión, 2 GPS con temperatura) |
| Reglas | 6 (temperatura de conservación, de congelación, de frío farmacéutico y de transporte; horas de compresor; calibración de sondas) |
| FSM | 2 (estado de cámara frigorífica, estado de transporte refrigerado) |
| Moneda | UYU, USD |
| Regulación | MSP (frío farmacéutico) |

### Qué valida este seed

- Monitoreo continuo de temperatura (eventos cada 5–15 min)
- Alertas de umbral con criticidad alta (excursión de temperatura = pérdida de producto)
- Dos FSMs diferentes para tipos de activo distintos (estático vs. móvil)
- Normativa sanitaria (MSP, frío farmacéutico) como driver de alertas
- Activos instalados en ubicaciones de terceros (clientes)

---

## 4. Seed 03 — Extinfire SRL

**Archivo:** `seeds/seed-03-extinfire.seed` (se carga con `seeds/seed-03-extinfire.py`)
**Vertical:** Mantenimiento de extintores y sistemas contra incendio
**Mercado:** Uruguay — departamentos de Montevideo, Canelones y Maldonado
**Empresa:** Extinfire SRL, empresa montevideana de mantenimiento de
extintores registrada ante la Dirección Nacional de Bomberos (DNB).

### Perfil de la empresa

Extinfire presta servicio de mantenimiento, recarga y ensayo hidrostático
de extintores a clientes comerciales, industriales, educativos, hoteleros
y residenciales. La empresa no es dueña de los extintores — los
propietarios de los inmuebles son los responsables legales. Extinfire
gestiona el ciclo de vida de cada extintor bajo contrato de servicio.

### Marco regulatorio

El seed modela con fidelidad el marco normativo uruguayo:

- **Decreto 372/023** y **Decreto 333/2000**: prevención y defensa contra incendio
- **UNIT 598:2017**: extintores de polvo, vida útil máxima de 20 años
- **UNIT 586**: extintores de CO2
- **UNIT 607**: inspección y mantenimiento
- **UNIT 1221:2015**: extintores Clase K (cocinas)
- **IT-04 DNB**: instrucción técnica de instalación

### Ciclos de mantenimiento modelados

| Intervención | Polvo ABC | CO2 | Espuma / Clase K |
|---|---|---|---|
| Inspección visual | Trimestral | Trimestral | Trimestral |
| Inspección anual | 1 año | 1 año | 1 año |
| Recarga | 2 años | 2 años | 1 año |
| Ensayo hidrostático | 4 años | 5 años | 2 años |
| Vida útil máxima | 20 años | — | — |

### Características del seed

| Dimensión | Valor |
|---|---|
| Activos | 113 |
| Tipos de activo | 7 (polvo ABC, CO2, espuma AFFF, Clase K, rodante, BIE, panel detección) |
| Marcas | 7 (Fadesa, Drago, Melisam, Amerex, Kidde, Yamato, Grauser) |
| Modelos | 17 utilizados de 21 definidos |
| Ubicaciones | 11 (8 clientes + taller + depósito + taller Extinfire) |
| Custodios | 8 (Extinfire + 7 clientes) |
| Sensores | 6 (2 QR, 2 BLE presión, 1 NFC tamper, 1 gateway Modbus) |
| Reglas | 7 (recarga, hidrostático, inspección, vida útil) |
| FSM | 2 (ciclo de vida extintor + habilitación DNB del sitio) |
| Moneda | UYU, USD |
| Años de fabricación | 17 distintos (2005–2025) |

### Casos borde ejercitados

| Caso | Cantidad | Modelado como |
|---|---|---|
| Próximo a fin de vida útil (20 años) | 6 | `fecha_expiracion` activa |
| Retirado / destruido (Desguace) | 3 | `status: inactive`, `motivo_baja: Desguace` |
| Extraviado / robado | 1 | `status: lost` |
| Reprobado en hidrostático | 1 | `status: inactive`, en taller |
| Precio en USD (importación) | 1 | `moneda_id: USD` |
| Sello de seguridad roto | 1 | Nombre indica condición |
| En taller para recarga | 3 | `status: maintenance`, ubicación taller |
| En taller para hidrostático | 2 | `status: maintenance`, ubicación taller |
| Con garantía vigente | 3 | `fin_garantia` definido |

### Modelo de custodia

La custodia sigue un modelo invertido respecto a los demás seeds:

- Los **clientes** son dueños de los extintores (`calidad_tenencia: Propiedad`)
- Extinfire tiene custodia **temporal** cuando los extintores están en el taller
  (la ubicación cambia a `TALLER-EF`, el custodio sigue siendo el cliente)
- Solo los extintores **destruidos o reprobados** transfieren custodia a
  Extinfire (responsabilidad de destrucción y reporte a DNB)

---

## 5. Seed 04 — VendiBeria S.L.

**Archivo:** `seeds/seed-04-vendiberia.seed` (se carga con `seeds/seed-04-vendiberia.py`)
**Documento complementario:** `seeds/SEED-04-COMPANION.md`
**Vertical:** Operación de máquinas expendedoras (vending)
**Mercado:** España — Comunidad de Madrid y Castilla-La Mancha
**Empresa:** VendiBeria S.L., operador de vending con sede en Madrid,
miembro de ANEDA, certificación AQS.

### Perfil de la empresa

VendiBeria opera ~95 activos rastreados (máquinas + terminales de pago)
distribuidos en oficinas corporativas, hospitales, universidades, estaciones
de transporte, parques industriales, centros comerciales y un instituto de
educación secundaria. Dos rutas de servicio diarias (Madrid Centro y
Madrid Sur-Toledo), tres reponedores, un técnico SAT dedicado.

### Por qué este vertical es estructuralmente diferente

La máquina expendedora es el primer tipo de activo en los seeds que
**genera ingresos evento-a-evento**. Cada dispensación es simultáneamente
un evento de inventario, una transacción financiera, y (para máquinas
de comida) una acción relevante para la seguridad alimentaria. Esto hace
que la cadena de eventos sea mucho más rica que en los otros tres seeds.

Además, el **terminal de pago** se modela como activo separable: tiene
número de serie propio, ciclo de mantenimiento independiente, y puede
migrar entre máquinas. Ningún otro seed tiene un activo que existe como
entidad independiente pero se instala físicamente dentro de otro activo.

### Marcas y modelos

Todas las marcas y modelos son reales y verificados en el mercado español:

| Marca | Origen | Especialidad |
|---|---|---|
| Azkoyen | Peralta, Navarra, España | Café, tabaco — líder español |
| Jofemar | Peralta, Navarra, España | Snacks, bebidas, café |
| Necta (Evoca) | Valbrembo, Italia | Café, snacks — mayor facturación en España |
| Bianchi | Bérgamo, Italia | Café — serie LEI 700 |
| SandenVendo | EE.UU./Japón | Snacks, bebidas frías — G-Snack, G-Drink |
| Saeco (Evoca) | Gaggio Montano, Italia | Café compacto — Phedra EVO |
| Nayax | Israel | Terminales cashless — VPOS Touch/Fusion |
| Caslab | España | Telemetría — dispositivo MÁSTER |

### Características del seed

| Dimensión | Valor |
|---|---|
| Activos | 94 |
| Tipos de activo | 7 (café, snacks, bebidas frías, combo, comida fresca, terminal de pago, micromarket) |
| Marcas | 8 (6 fabricantes de máquinas + Nayax + Caslab) |
| Modelos | 16 utilizados de 17 definidos |
| Ubicaciones | 11 (10 clientes + almacén VendiBeria) |
| Custodios | 11 (VendiBeria + 10 clientes) |
| Sensores | 8 (2 QR, 4 Caslab telemetría, 2 sondas temperatura) |
| Reglas | 6 (temperatura, visitas, limpieza, terminal) |
| FSM | 2 (estado operativo + ciclo de vida) |
| Moneda | EUR |
| Años de fabricación | 12 distintos (2014–2025) |
| Conectividad | ~30% gateway (Caslab), ~70% QR-only |

### Distribución de activos por tipo

| Tipo | Cantidad | Proporción | Notas |
|---|---|---|---|
| Máquina de Café | 32 | 34% | Domina, refleja mercado español (~74% de ventas) |
| Máquina de Snacks | 17 | 18% | Segundo más común |
| Máquina de Bebidas Frías | 16 | 17% | Refrigerada, pico estacional verano |
| Terminal de Pago Cashless | 12 | 13% | Activo separable, migrable entre máquinas |
| Máquina Combo | 9 | 10% | Ubicaciones pequeñas |
| Máquina de Comida Fresca | 5 | 5% | Segmento en crecimiento, máxima exigencia APPCC |
| Micromarket | 3 | 3% | Formato premium, nuevo |

### Distribución por sitio cliente

| Sitio | Activos | Carácter |
|---|---|---|
| Telefónica — Distrito C | 21 | Campus corporativo, mayor cliente, micromarket |
| Hospital de Toledo | 10 | APPCC estricto, 24/7, comida fresca |
| UC3M Getafe | 10 | Universidad, estacional, tráfico estudiantil |
| Deloitte — Castellana | 9 | Corporativo premium, micromarket, opciones saludables |
| Xanadú — Arroyomolinos | 9 | Centro comercial, picos fin de semana |
| Hospital Puerta de Hierro | 9 | APPCC estricto, 24/7, comida fresca |
| Atocha — Renfe | 8 | Hub de transporte, altísimo tráfico, riesgo vandalismo |
| Illescas — Pol. Industrial | 6 | Fábrica, consumo por turnos |
| Almacén VendiBeria | 6 | Reparación (3), retirada (1), demo (1), robada (1) |
| Acciona — Alcobendas | 5 | Sede corporativa, tráfico medio |
| IES Cervantes — Alcalá | 3 | Instituto público, flota antigua, solo monedas |

### Casos borde ejercitados

| Caso | Modelado como |
|---|---|
| Máquina antigua sin telemetría (2015–2017) | QR-only, criticidad Baja, sin terminal |
| Máquina nueva con Caslab MÁSTER | Gateway, sensor vinculado |
| Máquinas en taller (3 averías distintas) | `status: maintenance`, diferentes diagnósticos |
| Máquina robada | `status: lost`, `motivo_baja: Robo` |
| Máquina retirada (depreciada) | `status: inactive`, `motivo_baja: Desguace` |
| Máquina en leasing del fabricante | `calidad_tenencia: Alquiler` |
| Comida fresca con producto próximo a caducar | Alerta runtime activa |
| Terminal de pago como activo separable | Tipo independiente con serie y firmware propios |
| Micromarket (formato nuevo) | Tipo con cámaras y checkout |
| Múltiples garantías vigentes | 12 activos con `fin_garantia` |
| Hospital con requisitos APPCC estrictos | Umbral temperatura 6 °C (vs 8 °C bebidas) |

### Regulación

| Norma | Aplicación |
|---|---|
| APPCC / HACCP | Monitoreo de temperatura obligatorio en máquinas refrigeradas |
| Ley 6/2025 (Galicia) | Restricción de venta de bebidas energéticas a menores |
| Real Decreto (borrador) | Regulación de alimentación saludable en hospitales |
| PSD2 / PCI DSS | Pagos cashless (gestionado por proveedor del terminal) |
| Certificación AQS (ANEDA) | Calidad del servicio de vending |

---

## 6. Seed 07 — Beira Magalhães Hotéis e Resorts Ltda.

**Archivo:** `seeds/seed-07-beiramagalhaes.seed` (se carga con `seeds/seed-07-beiramagalhaes.py`)
**Vertical:** Hotelería — mantenimiento de hoteles, resorts y posadas
**Mercado:** Brasil — estado de Bahía
**Idioma del contenido:** Portugués
**Empresa:** Beira Magalhães Hotéis e Resorts Ltda., grupo hotelero con
sede en Salvador.

### Perfil de la empresa

Beira Magalhães opera un hotel urbano en Salvador, dos resorts de playa
(Praia do Forte y Arraial d'Ajuda) y dos posadas (Morro de São Paulo y
Lençóis), con una lavandería central que atiende a todos. Su equipo de
ingeniería mantiene lo que un huésped no ve y nota enseguida cuando falla:
la climatización, el agua caliente, los ascensores, la cocina, la
lavandería, los generadores y las vans de traslado.

### Características del seed

| Dimensión | Valor |
|---|---|
| Activos | 16 |
| Tipos de activo | 7 (climatización, cocina industrial, ascensor, lavandería, grupo generador, caldera y calentamiento, vehículo de apoyo) |
| Marcas | 9 (Carrier, Daikin, Rational, Hobart, Atlas Schindler, Electrolux Professional, Cummins, Mercedes-Benz, Bosch) |
| Modelos | 15 |
| Ubicaciones | 7 (hotel, central de utilidades, lavandería central, dos resorts, dos posadas) |
| Custodios | 5 (Beira Magalhães + 4: climatización, ascensores, arrendadora de vehículos y una persona física) |
| Sensores | 10 (3 GPS, 2 controladores de chiller, 2 de generador, 2 sondas de temperatura, 1 monitor de ascensor) |
| Reglas | 5 (revisión de vans, horas de climatización, geocerca, GPS sin reporte, prueba mensual de generador) |
| FSM | 2 (ciclo de vida de vehículos, ciclo de vida de equipos del edificio) |
| Moneda | BRL, USD, EUR |
| Usuarios | 17 (dos cuentas admin propias; las demás, las de todos los seeds) |

### Qué valida este seed

- La interfaz completa en portugués, con datos escritos en ese idioma y no
  traducidos de otro seed
- Un sector de servicios: los activos son instalaciones de un edificio, no
  equipos de obra ni de un cliente
- Un caso agrupado con sentido operativo en su seed de incidencias: la
  caldera se detiene y la cocina y la lavandería reportan la falta de agua
  caliente
- Un equipo en comodato con vencimiento y otro con la garantía por vencer
- Contratistas de mantenimiento como custodios (climatización y ascensores)

Es el único seed escrito directamente como archivo `.seed`; los seis
anteriores se convirtieron de scripts de Python.

---

## 7. Comparación entre Seeds

### Escala y complejidad

| Seed | Activos | Tipos | Marcas | Modelos | Ubicaciones | Custodios | Sensores | Reglas | FSMs | Eventos |
|---|---|---|---|---|---|---|---|---|---|---|
| 01 — Constructec | 16 | 7 | 8 | 15 | 7 | 5 | 10 | 5 | 2 | 6 |
| 02 — FrigoCold | 18 | 7 | 10 | 18 | 10 | 6 | 20 | 6 | 2 | 8 |
| 03 — Extinfire | 113 | 7 | 7 | 21 | 11 | 8 | 6 | 7 | 2 | 8 |
| 04 — VendiBeria | 94 | 7 | 8 | 17 | 11 | 11 | 8 | 6 | 2 | 9 |
| 07 — Beira Magalhães | 16 | 7 | 9 | 15 | 7 | 5 | 10 | 5 | 2 | 6 |

Los seeds 05 y 06 tienen las cifras del seed 01. Todos crean 16 usuarios
(17 los seeds 05, 06 y 07, que tienen dos cuentas admin propias).

### Características diferenciadoras

| Característica | 01 | 02 | 03 | 04 | 07 |
|---|---|---|---|---|---|
| Activo genera ingresos | No | No | No | Si | No |
| Monitoreo temperatura | Parcial | Si | No | Si | Parcial |
| Regulación estricta | No | Si | Si | Parcial | Parcial |
| GPS / ubicación móvil | Si | Parcial | No | No | Parcial |
| Activos separables | No | No | No | Si | No |
| Modelo custodia invertido | No | No | Si | No | No |
| Eventos comerciales (ventas, EUR) | No | No | No | Si | No |
| Ciclo vida regulado (20 años) | No | No | Si | No | No |
| Mapa habilitado | Si | Si | No | Si | Si |

### Mercados y monedas

| Seed | País | Regiones | Monedas |
|---|---|---|---|
| 01 — Constructec | Uruguay | Metropolitana, Norte, Sur, Este, Centro | UYU, USD, EUR |
| 02 — FrigoCold | Uruguay | Metropolitana, Litoral Oeste, Norte, Sur, Este | UYU, USD |
| 03 — Extinfire | Uruguay | Metropolitana, Canelones, Maldonado | UYU, USD |
| 04 — VendiBeria | España | Madrid Centro, Madrid Sur, Castilla-La Mancha | EUR |
| 05 — Fairborne | Estados Unidos (noroeste del Pacífico) | Las del seed 01, en inglés | Las del seed 01 |
| 06 — Asahi Kenki | Japón (región de Kanto) | Las del seed 01, en japonés | Las del seed 01 |
| 07 — Beira Magalhães | Brasil (Bahía) | Salvador e Região Metropolitana, Litoral Norte, Costa do Dendê, Chapada Diamantina, Costa do Descobrimento | BRL, USD, EUR |

---

## 8. Notas Técnicas

### Orden de las entidades

Cada archivo lista sus 20 tipos de entidad en el orden en que dependen
unos de otros, que es también el orden en que se leen mejor:

 1. Regiones (`regions`)
 2. Monedas (`currencies`)
 3. Centros de costo (`cost_centers`)
 4. Marcas (`brands`)
 5. Modelos (`models`; dependen de las marcas)
 6. Custodios (`custodians`)
 7. Tipos de alerta (`alert_types`)
 8. Definiciones de propiedades (`prop_definitions`)
 9. Ubicaciones (`locations`; dependen de las regiones)
10. Máquinas de estados (`fsm_machines`)
11. Tipos de activo (`asset_types`, con su máquina de estados)
12. Activos (`assets`, con su configuración de servicio)
13. Asociaciones de propiedades (`prop_associations`)
14. Sensores (`sensors`)
15. Usuarios (`users`)
16. Configuración del tenant (`tenant_settings`)
17. Reglas (`rules`)
18. Vinculación de sensores a activos (`sensor_bindings`)
19. Alertas en tiempo real (`alerts`)
20. Eventos (`events`)

Cada registro lleva su identificador, de modo que las referencias entre
entidades (`type_id`, `brand_id`, `alert_id` y las demás) son los mismos
números en cualquier instancia.

### Fechas relativas

Las fechas que dependen del día de la carga no están fijas en los archivos:
se escriben como desplazamientos, por ejemplo `{"$date": "+180d"}` para un
vencimiento a seis meses o `{"$now": "-2h"}` para una alerta de hace dos
horas, y se resuelven al cargar. Una demostración cargada hoy o dentro de
un año muestra las mismas garantías vigentes y los mismos servicios
próximos. `--as-of 2026-11-17` fija el día de referencia.

### Validación

Cada seed ha sido verificado contra los schemas de validación definidos
en `internal/validation/schemas.go`. Las verificaciones incluyen:

- Valores de enumeración (status, criticidad, tipo_sujeto, calidad_tenencia,
  motivo_baja, trigger_type, condition_type, action_type, obligatoriedad,
  entidad_destino, role)
- Longitudes de cadena (nombre_corto, descripcion, tag_tecnico, serie_1,
  email, telefono, direccion)
- Formatos de fecha (YYYY-MM-DD, máximo 10 caracteres)
- Patrones de slug (`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
- Colores hexadecimales (`^#[0-9A-Fa-f]{6}$`)
- Tipos de campo en templates (text, number, boolean, date, datetime,
  select, ref)
- Unicidad de tag_tecnico y números de serie dentro de cada seed
- Integridad referencial: todas las referencias a activos, sensores,
  marcas y modelos resuelven a entidades definidas

### Usuarios de acceso

Todos los seeds crean una cuenta por rol, todas en el tenant 1 (cada seed
carga una sola empresa en una instancia vacía):

| Email | Contraseña | Rol |
|---|---|---|
| hl@example.com | holaquetal | admin |
| dev@ual.li | holaquetal | developer |
| manager@example.com | holaquetal | manager |
| viewer@example.com | holaquetal | viewer |
| (equipo de desarrollo: 12 usuarios) | holaquetal | operator |

La cuenta `developer` solo ve su módulo si la instancia arranca con
`SEAM_ENABLE_DEVELOPER`. Las cuentas `manager` y `viewer` sirven para ver la
aplicación con ese rol sin usar la opción «ver como» del módulo Developer. En
los seeds 05, 06 y 07 las cuentas admin son otras, con su propia contraseña; las
demás son las de la tabla:

| Seed | Cuentas admin | Contraseña |
|---|---|---|
| 05 — Fairborne | admin@fairborne-equipment.example, ops@fairborne-equipment.example | letsgetstarted |
| 06 — Asahi Kenki | admin@asahi-kenki.example, ops@asahi-kenki.example | hajimemashou |
| 07 — Beira Magalhães | admin@beiramagalhaes.example, ops@beiramagalhaes.example | letsgetstarted |

Cada seed imprime sus cuentas al terminar.

---

## 9. Ejecución

### Prerrequisitos

- Python 3.8 o superior. No hace falta instalar ningún módulo.
- Instancia de xolu corriendo, accesible y vacía

### Ejecución

Con el lanzador no hace falta cargar nada a mano. La primera vez que se
usa un directorio de datos, `make launch` pregunta con qué empezar, y un
seed viene con sus complementos (sección 10):

```bash
make launch SEED=03-extinfire     # ese seed, con órdenes de trabajo, incidencias y contabilidad
make dev                          # todos los módulos encendidos, sobre el seed 01
make demo SEED=03-extinfire PLATFORM=linux    # un paquete de demo con esos mismos datos
```

Para cargarlos sobre un xolu que ya está corriendo, se carga un solo seed
por instancia:

```bash
# Seed 01 — Constructec (maquinaria e instalaciones, Uruguay)
python3 seeds/seed-01-constructec.py --url http://localhost:9090

# Seed 02 — FrigoCold (cadena de frío, Uruguay)
python3 seeds/seed-02-frigocold.py --url http://localhost:9090

# Seed 03 — Extinfire (extintores, Uruguay)
python3 seeds/seed-03-extinfire.py --url http://localhost:9090

# Seed 04 — VendiBeria (vending, España)
python3 seeds/seed-04-vendiberia.py --url http://localhost:9090

# Seed 05 — Fairborne (el seed 01 en inglés)
python3 seeds/seed-05-fairborne.py --url http://localhost:9090

# Seed 06 — Asahi Kenki (el seed 01 en japonés)
python3 seeds/seed-06-asahikenki.py --url http://localhost:9090

# Seed 07 — Beira Magalhães (hotelería, Brasil, en portugués)
python3 seeds/seed-07-beiramagalhaes.py --url http://localhost:9090
```

### Qué pasa si la instancia no está vacía

| Situación | Resultado |
|---|---|
| La instancia tiene otro seed, o datos que ninguna carga registró | La carga se rechaza y no se escribe nada. `--allow-existing` la permite. |
| La instancia ya tiene este mismo seed | Los registros que existen se dejan como están y se cargan los que falten. Así se retoma una carga interrumpida. |
| Lo mismo, con `--replace` | Todos los registros del seed se escriben de nuevo, y se pierde lo que se haya editado en ellos. |

Los registros que un usuario agregó nunca se tocan.

### Comparar una instancia con su seed

```bash
python3 seeds/seed-01-constructec.py --url http://localhost:9090 --check
```

No escribe nada: informa qué registros faltan, cuáles difieren y cuáles
sobran, y termina con código 1 si hay diferencias.

### Modo prueba (sin enviar datos)

```bash
python3 seeds/seed-04-vendiberia.py --dry-run
```

Imprime las peticiones HTTP que realizaría sin enviarlas. Útil para
verificar la estructura de datos antes de poblar una instancia.

### El comando general

Los siete comandos anteriores son atajos de `seeds/seed.py`, que además
lista los seeds, muestra qué contiene cada uno y vuelca una instancia a un
archivo `.seed`:

```bash
python3 seeds/seed.py list
python3 seeds/seed.py info 03-extinfire
python3 seeds/seed.py load 03-extinfire --url http://localhost:9090
python3 seeds/seed.py dump --url http://localhost:9090 -o copia.seed
```

---

## 10. Complementos

Los complementos son seeds que agregan datos a lo que la instancia ya
tiene. Están en un directorio por módulo y son archivos `.seed` como los
siete principales, con una diferencia: ningún registro lleva id. Cada uno
se busca por un campo propio (un código, un email, una etiqueta); si
existe se deja como está, y si no, se crea. Por eso se pueden cargar más de
una vez, y sobre datos que cambiaron.

| Complemento | Nombre para `seed.py load` | Se carga sobre |
|---|---|---|
| Órdenes de trabajo | `wo-actions`, `wo-templates` | Cualquier seed, o una instancia vacía |
| Incidencias | `issues-01-constructec` … `issues-07-beiramagalhaes` | El seed del mismo número |
| Contabilidad | `accounting` | Cualquier seed, o una instancia vacía |

Los tres módulos vienen apagados en Seam, y cada complemento dice en su
encabezado para cuál es (`x.seam.module`). `make launch SEED=...`,
`make dev` y `make demo` encienden los módulos de los datos que cargan. Si
cargas un complemento a mano con `seed.py load`, enciende tú su módulo:

| Complemento | Módulo | Variable de Seam |
|---|---|---|
| Órdenes de trabajo | `work_orders` | `SEAM_ENABLE_WORK_ORDERS=1` |
| Incidencias | `issues` | `SEAM_ENABLE_ISSUES=1` |
| Contabilidad | `accounting` | `SEAM_ENABLE_ACCOUNTING=1` |

### Seeds de órdenes de trabajo

**Archivos:** `seeds/workorders/seed-wo-actions.seed`,
`seeds/workorders/seed-wo-templates.seed` (y los comandos
`seed-wo-actions.py` y `seed-wo-templates.py`, que los cargan)

Para el módulo de órdenes de trabajo. No necesitan los datos de ningún
seed.

| Seed | Qué crea |
|---|---|
| `wo-actions` | Cuatro acciones (`wo_actions`) para probar el mapeo de parámetros de las plantillas |
| `wo-templates` | Tres plantillas de orden de trabajo y lo que necesitan: un tipo de activo, activos, una unidad, un tipo de consumible y consumibles. Crea las acciones que falten |

Los registros que ya existen se reutilizan y las plantillas no se modifican
en su lugar (`--replace`, o `--update-existing`, las vuelve a escribir). No
crean almacenes, saldos de inventario, movimientos, usuarios ni órdenes.
Las reglas y acciones son ejemplos de configuración: el módulo no ejecuta
su lógica de negocio.

```bash
python3 seeds/workorders/seed-wo-templates.py --url http://localhost:9090
```

### Seeds de incidencias

**Archivos:** `seeds/issues/seed-issues-NN-nombre.seed`, uno por cada seed
principal, en su idioma.

Cada uno exige que su seed esté cargado y busca en él los usuarios, tipos
de activo y activos que usa; si falta alguno, la carga se detiene sin
escribir. xolu tiene que correr con `XOLU_API_V2_ENABLED=true`: el ciclo de
vida de una incidencia es una máquina de estados.

| Qué agrega | Cantidad | Detalle |
|---|---|---|
| Incidencias | 12 | Dos reportadas, una en triaje, cinco en progreso, una en espera, una resuelta, una cerrada y una rechazada con su motivo; por la app y por código QR; con SLA vigente y vencido |
| Caso | 1 | La incidencia ISS-10 agrupa a ISS-11 e ISS-12, que siguen su estado |
| Plantillas | 2 | Campos adicionales por tipo de activo; una tiene tiempos de SLA propios y la otra dos versiones |
| Asignaciones | 2 | Cada plantilla, a un tipo de activo |
| Códigos QR | 4 | Tres activos; uno tiene un código revocado y otro vigente |

Cada incidencia tiene su máquina de estados, y los pasos que la llevaron a
su estado se hacen durante la carga: el historial muestra a las personas
correctas con la hora de la carga. Las fechas de la incidencia (creación,
vencimientos, primera respuesta) son relativas al momento de la carga.

```bash
python3 seeds/seed.py load 01-constructec --url http://localhost:9090
python3 seeds/seed.py load issues-01-constructec --url http://localhost:9090
```

### Seed de contabilidad

**Archivo:** `seeds/accounting/seed-accounting.seed`

Para el módulo de Contabilidad y Administración
(`docs/ACCOUNTING_ADMIN_MODULE.md`), que hay que encender en Seam
(`SEAM_ENABLE_ACCOUNTING=1`). Son los libros de la empresa que vende Seam,
no los de un cliente, así que no depende de ningún seed: sus tenants son
las empresas de los seeds 01 a 06 (la del seed 07 no figura en esos libros).

| Qué agrega | Cantidad | Detalle |
|---|---|---|
| Usuarios | 6 | Un administrativo, un contador, tres ejecutivos de cuentas y la cuenta de desarrollador |
| Ejecutivos de cuentas | 6 | Los dos virtuales, dos autorizados (uno cobra por transferencia y otro por PayPal), uno registrado que espera autorización y el de la cuenta de desarrollador |
| Tenants | 8 | Las empresas de los seeds 01 a 06, una filial en Paraguay y el tenant de prueba del desarrollador (Sandbox Demo Co.) |
| Ítems facturables | 43 | Julio a noviembre de 2026; noviembre tiene pendientes y cobranzas en curso |
| Asientos | 81 | Cobranzas, comisiones, pagos de liquidaciones y asientos manuales, uno de ellos revertido |
| Líneas de comisión | 34 | 10 % para los ejecutivos reales (más un bono por alta), 5 % para Seam AE |
| Liquidaciones | 5 | Cuatro pagadas y una con la factura cargada, a la espera del pago |
| Cierres mensuales | 4 | Julio a octubre |
| Reglas de fórmula | 5 | Tres de comisión y dos de ajuste de precio (una apagada) |

| Usuario | Contraseña | Rol |
|---|---|---|
| `accounting.staff@example.com` | `holaquetal` | `admin_staff` |
| `accountant@example.com` | `holaquetal` | `accountant` |
| `marta.rivas@example.com` | `holaquetal` | `account_executive` |
| `diego.suarez@example.com` | `holaquetal` | `account_executive` |
| `paula.mendez@example.com` | `holaquetal` | `account_executive` (espera autorización) |
| `dev@ual.li` | `holaquetal` | `developer` |

`dev@ual.li` es la misma cuenta de desarrollador de los siete seeds: si uno
de ellos está cargado, este la encuentra por su email y la deja como está.
Con el módulo Developer encendido (`SEAM_ENABLE_DEVELOPER=1`) puede ver la
aplicación como cada uno de los roles de arriba («Ver como un rol», en
`/developer`). Para que al verla como ejecutivo de cuentas el portal tenga
contenido, la cuenta es además un ejecutivo autorizado con un cliente
propio, Sandbox Demo Co.: cuatro cobranzas con su comisión, una liquidación
pagada y tres líneas sin liquidar. Sus cobranzas van aparte de las demás,
así que los libros de las otras empresas son los mismos con o sin él.
`make dev-accounting` arranca Contabilidad sola con ese módulo encendido.

El archivo no se escribe a mano. Lo genera `cmd/accounting-seedgen`
(`make seed-accounting`), que ejecuta las reglas del propio módulo sobre
cinco meses: cada cobranza genera su asiento y su comisión, cada
liquidación reclama sus líneas, cada cierre guarda los saldos del mes. Las
fechas de los libros son fijas. De cada factura y comprobante está el
registro y no el archivo, que Seam guarda en disco: al abrirlo responde
«no encontrado».

```bash
python3 seeds/seed.py load accounting --url http://localhost:9090
```

### Esquemas de entidades del seed 01

**Archivo:** `seeds/seed-01-constructec.schemas.seed`

El seed 01 acepta `--schemas`: junto con los datos registra en xolu un
esquema por entidad, con lo que xolu valida cada registro contra él. Es
opcional; sin la opción, xolu crea las entidades al recibir su primer
registro, como en los demás seeds.

```bash
python3 seeds/seed-01-constructec.py --url http://localhost:9090 --schemas
```

### Documento de dominio del seed 04

**Archivo:** `seeds/SEED-04-COMPANION.md`

Describe en inglés el vertical de máquinas expendedoras con más detalle que
la sección 5: el modelo de canales, la operación de rutas y la regulación.

---

## 11. Generar este documento

La fuente es `seeds/SEEDS-CATALOGO.md`. De ella se generan una página HTML
autocontenida y un PDF:

```bash
python3 seeds/md2pdf.py
```

El script usa solo la biblioteca estándar de Python. Escribe siempre
`SEEDS-CATALOGO.html`, que se puede abrir e imprimir desde cualquier
navegador, y escribe `SEEDS-CATALOGO.pdf` cuando encuentra Chrome, Chromium
o Edge instalados, en Linux o en macOS. Con `--html` genera solo la página.
