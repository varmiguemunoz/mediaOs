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

type VideoScript struct {
	Title                    string   `json:"title"`
	Script                   string   `json:"script"`
	CaptionInstagram         string   `json:"caption_instagram"`
	CaptionFacebook          string   `json:"caption_facebook"`
	CaptionLinkedIn          string   `json:"caption_linkedin"`
	CaptionTikTok            string   `json:"caption_tiktok"`
	Hashtags                 []string `json:"hashtags"`
	CTA                      string   `json:"cta"`
	EstimatedDurationSeconds int      `json:"estimated_duration_seconds"`
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

Estructura:
0-3s: Hook impactante
3-10s: Problema o pregunta
10-30s: Explicación o mini tutorial
30-36s: Resultado o beneficio
36-40s: CTA

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
  "estimated_duration_seconds": 40
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
