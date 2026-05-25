package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	goopenai "github.com/sashabaranov/go-openai"
	"github.com/varmiguemunoz/content-automation/internal/config"
)

type Client struct {
	inner *goopenai.Client
	model string
}

func New(cfg *config.Config) *Client {
	return &Client{
		inner: goopenai.NewClient(cfg.OpenAIKey),
		model: cfg.OpenAIModel,
	}
}

type ContentPlanItem struct {
	Date       string `json:"date"`
	Pillar     string `json:"pillar"`
	Topic      string `json:"topic"`
	Angle      string `json:"angle"`
	Format     string `json:"format"`
	Difficulty string `json:"difficulty"`
	CTA        string `json:"cta"`
}

type TimelineSegment struct {
	Start          int    `json:"start"`
	End            int    `json:"end"`
	Segment        string `json:"segment"`
	SpokenText     string `json:"spoken_text"`
	VisualType     string `json:"visual_type"`
	OverlayText    string `json:"overlay_text"`
	Code           string `json:"code"`
	Language       string `json:"language"`
	HighlightLines []int  `json:"highlight_lines"`
}

type Caption struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type VideoScript struct {
	Title                    string            `json:"title"`
	Script                   string            `json:"script"`
	CaptionInstagram         string            `json:"caption_instagram"`
	CaptionFacebook          string            `json:"caption_facebook"`
	CaptionLinkedIn          string            `json:"caption_linkedin"`
	CaptionTikTok            string            `json:"caption_tiktok"`
	Hashtags                 []string          `json:"hashtags"`
	CTA                      string            `json:"cta"`
	EstimatedDurationSeconds int               `json:"estimated_duration_seconds"`
	Timeline                 []TimelineSegment `json:"timeline"`
	Captions                 []Caption         `json:"captions"`
}

func (c *Client) GenerateContentPlan(ctx context.Context, days int, startDate string) ([]ContentPlanItem, error) {
	prompt := fmt.Sprintf(`Eres un estratega de contenido para Miguel, divulgador de tecnología, software, IA y automatización en Medellín, Colombia.

Genera un plan de contenido de %d días comenzando desde %s.

El contenido debe enfocarse en estos pilares (rota entre todos):
- AI para negocios
- Automatización con WhatsApp
- Backend real
- TypeScript avanzado
- Arquitectura de software
- Herramientas para founders técnicos

Audiencia: Founders, agencias, negocios medianos y programadores que quieren aplicar tecnología real.

Reglas:
- Varía el pilar cada día
- El tema debe ser específico y accionable, no genérico
- El ángulo debe indicar qué lo hace diferente
- El formato debe ser: "tutorial 40s", "caso real", "error común", "comparación rápida"
- La dificultad debe ser: beginner, intermediate, advanced
- El CTA debe ser corto y directo (máx 10 palabras)
- Todo en español

Devuelve ÚNICAMENTE un JSON array válido con exactamente %d objetos. Sin texto extra. Sin markdown.
Esquema por objeto:
{
  "date": "YYYY-MM-DD",
  "pillar": "",
  "topic": "",
  "angle": "",
  "format": "",
  "difficulty": "",
  "cta": ""
}`, days, startDate, days)

	resp, err := c.inner.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model: c.model,
		Messages: []goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.8,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = stripMarkdownJSON(content)

	var items []ContentPlanItem
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, fmt.Errorf("parsing plan JSON: %w\nraw: %s", err, content[:min(200, len(content))])
	}

	if len(items) != days {
		return nil, fmt.Errorf("expected %d items, got %d", days, len(items))
	}

	return items, nil
}

func (c *Client) GenerateVideoScript(ctx context.Context, topic, pillar, angle, cta string) (*VideoScript, error) {
	const maxRetries = 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		script, err := c.generateScript(ctx, topic, pillar, angle, cta, attempt)
		if err != nil {
			lastErr = err
			continue
		}

		words := len(strings.Fields(script.Script))
		if words < 85 || words > 110 {
			lastErr = fmt.Errorf("script tiene %d palabras, necesita entre 85 y 110", words)
			continue
		}

		script.EstimatedDurationSeconds = 40
		return script, nil
	}

	return nil, fmt.Errorf("no se pudo generar un script válido tras %d intentos: %w", maxRetries+1, lastErr)
}

func (c *Client) generateScript(ctx context.Context, topic, pillar, angle, cta string, attempt int) (*VideoScript, error) {
	wordInstruction := ""
	if attempt == 1 {
		wordInstruction = "\nIMPORTANTE: El script debe tener EXACTAMENTE entre 85 y 110 palabras. Cuenta las palabras antes de responder."
	} else if attempt == 2 {
		wordInstruction = "\nCRÍTICO: El script anterior falló por tener muy pocas o muchas palabras. DEBE tener entre 85 y 110 palabras. Esto es un requisito estricto."
	}

	prompt := fmt.Sprintf(`Eres un estratega de contenido para Miguel, divulgador de tecnología, software, IA y automatización.

Crea un tutorial interactivo de 40 segundos para redes sociales.

Audiencia: Founders, agencias, negocios medianos y programadores que quieren entender cómo aplicar tecnología real.

Tema: %s
Pilar: %s
Ángulo: %s
CTA objetivo: %s%s

Reglas:
- El video debe sentirse humano, claro y práctico
- No sonar académico ni usar relleno
- Abrir con un hook fuerte en los primeros 3 segundos
- Incluir una mini demostración mental o ejemplo concreto
- Cerrar con el CTA corto
- Duración estimada: 85 a 110 palabras exactas en el script
- Idioma: español

Estructura de los 5 segmentos del timeline (usa estos nombres exactos para "segment"):
  hook     → 0-3s:   Hook impactante
  problem  → 3-10s:  Problema o pregunta
  solution → 10-30s: Explicación o mini tutorial (aquí va el código si aplica)
  result   → 30-36s: Resultado o beneficio concreto
  cta      → 36-40s: CTA corto y directo

Reglas del timeline:
- "visual_type" debe ser uno de: "talking_head", "code_demo", "caption_only", "split_screen"
  * talking_head: solo el avatar hablando
  * code_demo: fragmento de código con syntax highlight (usa "code", "language", "highlight_lines")
  * caption_only: texto grande en pantalla sin avatar visible
  * split_screen: avatar + código o texto al lado
- "spoken_text": el texto exacto que dice el avatar en ese segmento (subconjunto del script)
- "overlay_text": texto corto que aparece en pantalla (máx 8 palabras), vacío si no aplica
- "code": código real y funcional si visual_type es code_demo o split_screen, vacío si no aplica
- "language": lenguaje del código (e.g. "typescript", "python", "bash"), vacío si no aplica
- "highlight_lines": array de números de línea a resaltar, vacío [] si no aplica

Reglas de las captions:
- Divide el script completo en frases cortas (máx 8 palabras cada una)
- Distribuye los timestamps uniformemente a lo largo de los 40 segundos
- El texto de cada caption es la frase exacta del script

Devuelve ÚNICAMENTE JSON válido. Sin texto extra. Sin markdown.
{
  "title": "",
  "script": "",
  "caption_instagram": "",
  "caption_facebook": "",
  "caption_linkedin": "",
  "caption_tiktok": "",
  "hashtags": [],
  "cta": "",
  "estimated_duration_seconds": 40,
  "timeline": [
    {
      "start": 0,
      "end": 3,
      "segment": "hook",
      "spoken_text": "",
      "visual_type": "talking_head",
      "overlay_text": "",
      "code": "",
      "language": "",
      "highlight_lines": []
    }
  ],
  "captions": [
    {"start": 0.0, "end": 3.5, "text": ""}
  ]
}`, topic, pillar, angle, cta, wordInstruction)

	resp, err := c.inner.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model: c.model,
		Messages: []goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = stripMarkdownJSON(content)

	var script VideoScript
	if err := json.Unmarshal([]byte(content), &script); err != nil {
		return nil, fmt.Errorf("parsing script JSON: %w", err)
	}

	if script.Script == "" {
		return nil, fmt.Errorf("script vacío en respuesta")
	}

	return &script, nil
}

type CompositionInput struct {
	Topic    string
	Pillar   string
	Timeline []TimelineSegment
	Captions []Caption
	Width    int
	Height   int
	FPS      int
}

func (c *Client) GenerateCompositionHTML(ctx context.Context, input CompositionInput) (string, error) {
	timelineJSON, err := json.Marshal(input.Timeline)
	if err != nil {
		return "", fmt.Errorf("serializing timeline: %w", err)
	}
	captionsJSON, err := json.Marshal(input.Captions)
	if err != nil {
		return "", fmt.Errorf("serializing captions: %w", err)
	}

	prompt := fmt.Sprintf(`Eres un experto en animaciones web y motion graphics para video.

Genera un archivo HTML completo y auto-contenido que será renderizado frame a frame por Puppeteer (HyperFrames) para producir un video MP4 de %dx%d píxeles a %d fps.

TEMA: %s
PILAR: %s

TIMELINE (los segmentos del video con sus tiempos exactos en segundos):
%s

CAPTIONS (subtítulos con sus tiempos exactos):
%s

REGLAS ESTRICTAS:
1. El HTML debe ser 100%% auto-contenido. Sin archivos externos. Todas las fuentes, estilos y scripts inline.
2. El body debe tener exactamente width=%dpx y height=%dpx. Sin scroll. overflow:hidden.
3. Las animaciones CSS/JS deben ser time-based (usando Date.now() o requestAnimationFrame) para sincronizarse con los segundos del timeline.
4. Cada segmento del timeline debe aparecer en su segundo "start" y desaparecer en su segundo "end".
5. El video dura exactamente 40 segundos. Después del segundo 40 todo puede desaparecer.

DISEÑO REQUERIDO:
- Fondo: dark (#0A0A0A o gradiente oscuro)
- Tipografía: system-ui o sans-serif, bold para títulos
- Colores de acento: cian (#00D4FF) y azul eléctrico (#4C6EF5)
- Cada segmento ocupa toda la pantalla con su contenido centrado verticalmente

TIPOS DE VISUAL (visual_type en el timeline):
- "talking_head": fondo animado sutil (gradiente suave o partículas), texto del overlay centrado en la parte superior
- "code_demo": editor de código oscuro (estilo VS Code), syntax highlighting manual con spans coloreados, animación de escritura línea por línea. highlight_lines se resaltan con fondo amarillo/verde.
- "caption_only": texto grande centrado (2-3 líneas máx), fondo sólido con color de acento
- "split_screen": mitad izquierda texto/overlay, mitad derecha código o diagrama

CAPTIONS (subtítulos):
- Mostrar siempre en la parte inferior (bottom: 80px), fuente blanca, fondo semitransparente oscuro
- Sincronizados con los timestamps del array "captions"
- Tamaño de fuente: 36px, max-width: 90%%, text-align: center

TRANSICIONES:
- fadeIn de 0.3s al inicio de cada segmento
- fadeOut de 0.3s antes de que termine cada segmento

Devuelve ÚNICAMENTE el código HTML completo. Sin explicaciones. Sin markdown. Sin bloques de código. Solo el HTML que empieza con <!DOCTYPE html>.`,
		input.Width, input.Height, input.FPS,
		input.Topic, input.Pillar,
		string(timelineJSON),
		string(captionsJSON),
		input.Width, input.Height,
		input.Width, input.Height,
	)

	resp, err := c.inner.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model: c.model,
		Messages: []goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.4,
	})
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}

	html := strings.TrimSpace(resp.Choices[0].Message.Content)
	html = strings.TrimPrefix(html, "```html")
	html = strings.TrimPrefix(html, "```")
	html = strings.TrimSuffix(html, "```")
	html = strings.TrimSpace(html)

	if !strings.HasPrefix(html, "<!DOCTYPE") && !strings.HasPrefix(html, "<html") {
		return "", fmt.Errorf("respuesta inesperada de OpenAI: no parece HTML válido")
	}

	return html, nil
}

func stripMarkdownJSON(s string) string {
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
