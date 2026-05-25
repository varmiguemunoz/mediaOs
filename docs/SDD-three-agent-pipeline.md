# SDD: Three-Agent Video Pipeline
**Sistema de automatización de contenido — Miguel Ángel Muñoz**
**Versión:** 1.0 | **Estado:** Diseño | **Fecha:** 2026-05-24

---

## 1. Visión General

El sistema actual genera un plan de contenido y produce un video de avatar (HeyGen). Esta evolución agrega dos agentes adicionales que corren en paralelo al agente HeyGen, produciendo un video compuesto final de mayor calidad para publicación manual.

### Objetivo
Dado un script con timeline estructurado, producir automáticamente un video de 40 segundos que combine:
- **Cara y voz de Miguel** (HeyGen)
- **Motion graphics, escenas técnicas y subtítulos** (HyperFrames)
- **Video final compuesto** con ambas capas sincronizadas (FFmpeg)

### Días de ejecución
El pipeline completo corre los **lunes y jueves** activado por el cron job existente.

---

## 2. Arquitectura del Sistema

```
CRON (Lun/Jue 8am)
        │
        ▼
content daily
        │
        ├─── [OpenAI] Genera script estructurado con timeline JSON
        │
        ├──────────────────────────────────────────┐
        │                                          │
        ▼                                          ▼
  AGENTE 1: HeyGen                    AGENTE 2: HyperFrames
  Avatar + Voice video                HTML composition → MP4
  (async, ~3-10 min)                  (local render, ~1-3 min)
        │                                          │
        │  (check-videos polling)                  │
        ▼                                          ▼
  heygen_video_url               hyperframes_video.mp4
        │                                          │
        └──────────────┬───────────────────────────┘
                       │
                       ▼
             AGENTE 3: Editor (FFmpeg)
             Composite final video
                       │
                       ▼
             final_video.mp4
             → WhatsApp notification
             → Publicación manual
```

### Ejecución paralela
- Agente 1 y Agente 2 corren **en paralelo** (goroutines en Go).
- Agente 2 es local y síncrono (~1-3 min). Termina primero.
- Agente 1 es async (HeyGen API, ~3-10 min). Polling via `check-videos`.
- Agente 3 se ejecuta **solo cuando ambos Agentes 1 y 2 tienen output listo**.

---

## 3. Definición de los Agentes

### Agente 1 — HeyGen Avatar Agent
**Responsabilidad:** Producir el video de cara y voz de Miguel.

| | |
|---|---|
| **Input** | script.text, avatar_id, voice_id |
| **Tool** | HeyGen REST API v2/video/generate |
| **Output** | heygen_video_url (MP4, 9:16, 40s) |
| **Duración** | 3-10 minutos (async, requiere polling) |
| **Estado en DB** | video_requested → rendered |

El video de HeyGen muestra únicamente el avatar hablando. No tiene overlays ni efectos.

---

### Agente 2 — HyperFrames Composition Agent
**Responsabilidad:** Producir el video de motion graphics, escenas técnicas y subtítulos sincronizados con el timeline del script.

| | |
|---|---|
| **Input** | timeline JSON del script, topic, pillar, captions |
| **Tool** | HyperFrames CLI (`npx hyperframes render`) |
| **Output** | composition.mp4 (local, 9:16, 40s) |
| **Duración** | 1-3 minutos (render local síncrono) |
| **Estado en DB** | composition_pending → composition_ready |

**Flujo interno del Agente 2:**
1. Recibe el timeline estructurado del script
2. Llama a OpenAI para generar el HTML de la composición HyperFrames
3. Escribe el archivo `composition.html` en disco
4. Ejecuta `npx hyperframes render composition.html --output composition.mp4`
5. Guarda la ruta del MP4 en DB

**Tipo de contenido que genera:**
- Segmento `hook` (0-3s): título animado con fondo dinámico
- Segmento `problem` (3-10s): texto animado con highlight del problema
- Segmento `solution` (10-30s): código o demo técnico con animación GSAP, highlight de líneas, arrows
- Segmento `result` (30-36s): card de resultado / beneficio con animación de entrada
- Segmento `cta` (36-40s): overlay de CTA con background branded
- **Subtítulos/captions** sincronizados durante todo el video (generados desde las captions del script)

---

### Agente 3 — Editor / Compositor Agent
**Responsabilidad:** Combinar el output del Agente 1 y Agente 2 en un video final.

| | |
|---|---|
| **Input** | heygen_video_url, composition.mp4 path |
| **Tool** | FFmpeg |
| **Output** | final_video.mp4 (local, 9:16, 40s) |
| **Duración** | 30-60 segundos |
| **Estado en DB** | edit_pending → edit_ready |

**Layout del video final:**
```
┌─────────────────┐
│                 │
│  HyperFrames    │  ← Fondo completo: motion graphics, código, overlays
│  Composition    │
│                 │
│  ┌───────────┐  │
│  │  Miguel   │  │  ← HeyGen avatar en esquina inferior derecha
│  │  (PiP)    │  │     (picture-in-picture, ~30% del ancho)
│  └───────────┘  │
└─────────────────┘
```

El comando FFmpeg que ejecuta el Agente 3:
```bash
ffmpeg \
  -i composition.mp4 \
  -i heygen_video.mp4 \
  -filter_complex "[1:v]scale=iw*0.3:-1[pip]; [0:v][pip]overlay=W-w-20:H-h-20" \
  -c:a copy \
  -shortest \
  final_video.mp4
```

> El layout es configurable. Alternativas: split screen (50/50), overlay completo, intercalado por segmento.

---

## 4. Cambios al Modelo de Datos

### Nuevas tablas

```sql
-- Trabajos de composición HyperFrames
CREATE TABLE composition_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    html_path TEXT DEFAULT '',
    video_path TEXT DEFAULT '',
    status TEXT DEFAULT 'composition_pending',
    -- status: composition_pending | rendering | composition_ready | failed
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- Trabajos de edición final
CREATE TABLE edit_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_plan_id INTEGER NOT NULL REFERENCES content_plans(id),
    heygen_video_url TEXT DEFAULT '',
    composition_video_path TEXT DEFAULT '',
    final_video_path TEXT DEFAULT '',
    status TEXT DEFAULT 'edit_pending',
    -- status: edit_pending | editing | edit_ready | failed
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
```

### Cambio en `generated_scripts`
El campo actual almacena el script como texto plano. Se agrega la columna `timeline_json` para guardar el timeline estructurado:

```sql
ALTER TABLE generated_scripts ADD COLUMN timeline_json TEXT DEFAULT '';
```

### Estado completo del pipeline en `content_plans`
Los estados existentes se amplían:

```
planned → script_generated → video_requested → rendered →
composition_pending → composition_ready →
edit_pending → edit_ready →
pending_approval → approved → published | rejected | failed
```

---

## 5. Formato del Timeline del Script

El prompt de generación de script se actualiza para producir este JSON estructurado adicional:

```json
{
  "title": "Cómo automatizar respuestas en WhatsApp con IA",
  "script": "texto completo del script para HeyGen...",
  "timeline": [
    {
      "start": 0,
      "end": 3,
      "segment": "hook",
      "spoken_text": "¿Tu negocio pierde leads por no responder a tiempo?",
      "visual_type": "title_card",
      "overlay_text": "¿Pierdes leads por no responder?",
      "style": "bold_centered"
    },
    {
      "start": 3,
      "end": 10,
      "segment": "problem",
      "spoken_text": "El 78% de los leads espera respuesta en menos de 5 minutos...",
      "visual_type": "stat_card",
      "overlay_text": "78% espera respuesta en < 5 min",
      "style": "highlight"
    },
    {
      "start": 10,
      "end": 30,
      "segment": "solution",
      "spoken_text": "Con un agente de IA en WhatsApp puedes...",
      "visual_type": "code_demo",
      "code": "const agent = new WhatsAppAgent({\n  model: 'gpt-4o',\n  tools: [leadCapture, calendar]\n});\nawait agent.connect();",
      "language": "typescript",
      "highlight_lines": [2, 3],
      "overlay_text": "Conecta tu agente en 4 líneas"
    },
    {
      "start": 30,
      "end": 36,
      "segment": "result",
      "spoken_text": "El agente responde leads 24/7 sin intervención humana.",
      "visual_type": "result_card",
      "overlay_text": "24/7 · Sin intervención humana · Leads capturados",
      "style": "success_green"
    },
    {
      "start": 36,
      "end": 40,
      "segment": "cta",
      "spoken_text": "Comenta AGENTE y te muestro el flujo completo.",
      "visual_type": "cta_card",
      "overlay_text": "Comenta AGENTE 👇",
      "style": "branded_cta"
    }
  ],
  "captions": [
    {"start": 0.0, "end": 2.5, "text": "¿Tu negocio pierde leads por no responder a tiempo?"},
    {"start": 3.0, "end": 8.0, "text": "El 78% de los leads espera respuesta en menos de 5 minutos"}
  ],
  "caption_instagram": "...",
  "caption_facebook": "...",
  "caption_linkedin": "...",
  "caption_tiktok": "...",
  "hashtags": [],
  "cta": "Comenta AGENTE",
  "estimated_duration_seconds": 40
}
```

### Tipos de visual disponibles
| `visual_type` | Descripción |
|---|---|
| `title_card` | Título grande centrado con fondo animado |
| `stat_card` | Estadística en grande con animación de entrada |
| `code_demo` | Bloque de código con syntax highlighting y animación línea por línea |
| `result_card` | Card de resultado con íconos y checkmarks |
| `cta_card` | Call to action con brand colors |
| `comparison` | Tabla before/after animada |
| `flow_diagram` | Diagrama de flujo simple animado |

---

## 6. Estructura de Archivos del Agente 2 (HyperFrames)

```
mediaos/
├── hyperframes/
│   ├── compositions/
│   │   └── {plan_id}/
│   │       ├── composition.html    ← generado por OpenAI
│   │       └── composition.mp4    ← output de HyperFrames
│   └── templates/
│       ├── base.html              ← estructura base HyperFrames
│       └── segments/
│           ├── title_card.html
│           ├── code_demo.html
│           ├── stat_card.html
│           ├── result_card.html
│           └── cta_card.html
├── edits/
│   └── {plan_id}/
│       └── final_video.mp4        ← output del Agente 3
```

---

## 7. Nuevos Paquetes Go

```
internal/
├── hyperframes/
│   └── runner.go      ← ejecuta npx hyperframes render como subprocess
├── editor/
│   └── ffmpeg.go      ← ensambla el video final con FFmpeg
├── services/
│   └── orchestrator.go ← coordina los 3 agentes con goroutines
└── db/
    └── db.go          ← métodos nuevos para composition_jobs y edit_jobs
```

---

## 8. Nuevos Comandos CLI

```bash
content daily          # ACTUALIZADO: lanza Agentes 1+2 en paralelo
content check-videos   # ACTUALIZADO: cuando HeyGen termina, lanza Agente 3
content compose [id]   # NUEVO: corre Agente 2 manualmente (HyperFrames)
content edit [id]      # NUEVO: corre Agente 3 manualmente (FFmpeg merge)
content status         # ACTUALIZADO: muestra estado de los 3 agentes
```

---

## 9. Plan de Implementación (Fases)

### Fase 1 — Actualizar el prompt y el schema (Día 1)
**Objetivo:** El script generado incluye el timeline JSON estructurado.

- Actualizar `internal/openai/client.go`: nuevo struct `VideoScript` con campo `Timeline []TimelineSegment`
- Actualizar el prompt de generación para que devuelva el timeline
- Agregar migración SQL `002_timeline_and_agents.sql`
- Verificar que el plan de contenido guardado incluye el timeline

**Entregable:** `content daily` guarda el timeline en DB. Compile + test manual.

---

### Fase 2 — Agente 2: HyperFrames (Día 2-3)
**Objetivo:** Dado un timeline, generar y renderizar la composición HTML a MP4.

- Crear `internal/hyperframes/runner.go`:
  - `GenerateCompositionHTML(script, timeline)` → llama OpenAI para escribir el HTML
  - `RenderComposition(htmlPath, outputPath)` → ejecuta `npx hyperframes render` como subprocess
- Crear los templates base en `hyperframes/templates/`
- Crear `internal/db/` métodos: `InsertCompositionJob`, `UpdateCompositionJob`, `GetCompositionJobByPlanID`
- Agregar comando `content compose [id]`
- Test: `content compose 1` → genera `hyperframes/compositions/1/composition.mp4`

**Requisitos del sistema:** Node.js >= 22, FFmpeg, `npx hyperframes` disponible.

**Entregable:** MP4 de composition funcionando de forma independiente.

---

### Fase 3 — Agente 3: Editor FFmpeg (Día 4)
**Objetivo:** Combinar HeyGen video + composition.mp4 en final_video.mp4.

- Crear `internal/editor/ffmpeg.go`:
  - `Composite(heygenURL, compositionPath, outputPath, layout)` 
  - Descarga el video de HeyGen a disco primero
  - Ejecuta FFmpeg con el filtro picture-in-picture
- Crear `internal/db/` métodos: `InsertEditJob`, `UpdateEditJob`
- Agregar comando `content edit [id]`
- Test: `content edit 1` → genera `edits/1/final_video.mp4`

**Entregable:** Video final compuesto funcionando de forma independiente.

---

### Fase 4 — Orquestador (Día 5)
**Objetivo:** `content daily` lanza Agentes 1 y 2 en paralelo. `check-videos` lanza Agente 3 al terminar HeyGen.

- Crear `internal/services/orchestrator.go`:
  - `RunParallel(cfg, db, plan, script)` → goroutines para Agente 1 + Agente 2
  - Usa `sync.WaitGroup` + canales de error
- Actualizar `content daily` para llamar al orquestador
- Actualizar `content check-videos`: cuando HeyGen termina y composition_ready existe → lanza `editor.Composite()`
- Actualizar `content status` para mostrar estado de los 3 agentes

**Entregable:** Pipeline completo de punta a punta.

---

### Fase 5 — Prompt HyperFrames refinado (Día 6-7)
**Objetivo:** El HTML generado por OpenAI para HyperFrames produce composiciones de alta calidad.

- Iterar sobre el prompt que genera el HTML de composición
- Probar los 7 tipos de visual (title_card, code_demo, stat_card, etc.)
- Ajustar duración, animaciones GSAP, captions sincronizados
- Crear templates reutilizables para cada tipo de segmento

**Entregable:** Galería de 5+ videos de prueba con diferentes pilares de contenido.

---

## 10. Requisitos de Sistema

| Componente | Requisito |
|---|---|
| Go | 1.22+ |
| Node.js | >= 22 (para HyperFrames CLI) |
| FFmpeg | Instalado y en PATH |
| HyperFrames | `npx hyperframes` disponible |
| SQLite | Via go-sqlite3 |
| HeyGen API | Key activa con avatares configurados |
| OpenAI API | GPT-4o para scripts + composiciones |
| EvolutionAPI | En VPS, para notificaciones WhatsApp |
| Disco | ~500MB por video (temporal, se limpia tras WhatsApp) |

### Setup de HyperFrames en Mac
```bash
# Instalar Node.js 22 si no lo tienes
brew install node@22

# Instalar FFmpeg si no lo tienes
brew install ffmpeg

# Instalar HyperFrames globalmente
npm install -g hyperframes

# Verificar
npx hyperframes --version
```

---

## 11. Variables de Entorno Nuevas

```env
# Rutas locales para output de agentes
HYPERFRAMES_DIR=./hyperframes
EDITS_DIR=./edits

# Layout del video final (pip | split | overlay)
EDITOR_LAYOUT=pip

# HyperFrames: dimensiones del video (9:16 para Reels/TikTok)
VIDEO_WIDTH=1080
VIDEO_HEIGHT=1920
VIDEO_FPS=30
```

---

## 12. Consideraciones de Diseño

### Por qué paralelo y no secuencial
El Agente 1 tarda 3-10 min (HeyGen async). El Agente 2 tarda 1-3 min (render local). Corriéndolos en paralelo, el tiempo total del pipeline es ~max(t1, t2) en lugar de t1+t2. En la práctica: el pipeline completo tarda ~10 min en lugar de ~13 min.

### Manejo de errores
- Si el Agente 1 falla: el Agente 2 ya tiene su output listo. El Agente 3 no puede correr. Estado: `heygen_failed`. WhatsApp notifica.
- Si el Agente 2 falla: el Agente 3 tampoco puede correr. Estado: `composition_failed`. El video de HeyGen se puede usar solo como fallback.
- Si el Agente 3 falla: se notifica por WhatsApp con el error y las URLs individuales de ambos videos.

### Limpieza de archivos
Los archivos temporales en `hyperframes/` y `edits/` se eliminan automáticamente 7 días después de que el video fue aprobado. El video final se guarda permanentemente hasta aprobación + publicación manual.

### Layout del video final
El layout PiP (picture-in-picture) es el default y el más apropiado para contenido técnico porque:
- El fondo completo muestra el código o la demostración
- El avatar de Miguel mantiene la presencia personal
- Los subtítulos del motion graphics son legibles sin ser tapados por el avatar

---

## 13. Flujo Completo del Día de Ejecución

```
8:00 AM  Cron lunes/jueves
         └─ content daily
              ├─ Consulta plan de hoy en DB
              ├─ Genera script + timeline via OpenAI
              ├─ Guarda GeneratedScript con timeline_json
              ├─ [goroutine A] Envía a HeyGen API → video_requested
              └─ [goroutine B] Genera HTML composition → renderiza → composition_ready

~8:03 AM  Agente 2 completa (composition.mp4 listo)
          Estado: composition_ready

~8:10 AM  content check-videos (polling cada 30 min)
          ├─ HeyGen responde: completed
          ├─ Descarga heygen_video.mp4
          ├─ Agente 3: FFmpeg composite
          └─ final_video.mp4 listo → WhatsApp notification

~8:11 AM  WhatsApp recibe:
          "🎬 Video listo: [tema]
           URL: [final_video path o link]
           APPROVE 1 | REJECT 1"

Miguel revisa, responde APPROVE
          └─ content check-approval detecta APPROVE
               └─ Estado: approved
                    └─ Miguel publica manualmente
```

---

*Documento generado como guía de implementación. Actualizar conforme avance la fase de desarrollo.*
