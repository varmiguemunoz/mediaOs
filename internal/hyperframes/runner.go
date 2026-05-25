package hyperframes

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/openai"
)

type Runner struct {
	cfg *config.Config
	ai  *openai.Client
}

type CompositionInput struct {
	PlanID   int64
	Topic    string
	Pillar   string
	Timeline []openai.TimelineSegment
	Captions []openai.Caption
}

func New(cfg *config.Config, ai *openai.Client) *Runner {
	return &Runner{cfg: cfg, ai: ai}
}

func (r *Runner) Compose(ctx context.Context, input CompositionInput, logWriter io.Writer) (htmlPath string, videoPath string, err error) {
	dir := filepath.Join(r.cfg.HyperFramesDir, "compositions", strconv.FormatInt(input.PlanID, 10))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("creando directorio de composición: %w", err)
	}

	htmlPath = filepath.Join(dir, "composition.html")
	videoPath = filepath.Join(dir, "composition.mp4")

	fmt.Fprintf(logWriter, "⏳ Generando HTML de composición con OpenAI...\n")
	html, err := r.ai.GenerateCompositionHTML(ctx, openai.CompositionInput{
		Topic:    input.Topic,
		Pillar:   input.Pillar,
		Timeline: input.Timeline,
		Captions: input.Captions,
		Width:    r.cfg.VideoWidth,
		Height:   r.cfg.VideoHeight,
		FPS:      r.cfg.VideoFPS,
	})
	if err != nil {
		return "", "", fmt.Errorf("generando HTML: %w", err)
	}

	if err = os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return "", "", fmt.Errorf("escribiendo HTML a disco: %w", err)
	}
	fmt.Fprintf(logWriter, "✅ HTML escrito: %s\n", htmlPath)

	fmt.Fprintf(logWriter, "⏳ Renderizando video con HyperFrames...\n")
	if err = r.renderVideo(htmlPath, videoPath, logWriter); err != nil {
		return htmlPath, "", fmt.Errorf("renderizando video: %w", err)
	}
	fmt.Fprintf(logWriter, "✅ Video renderizado: %s\n", videoPath)

	return htmlPath, videoPath, nil
}

func (r *Runner) renderVideo(htmlPath, videoPath string, logWriter io.Writer) error {
	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("resolviendo ruta absoluta: %w", err)
	}
	absVideo, err := filepath.Abs(videoPath)
	if err != nil {
		return fmt.Errorf("resolviendo ruta absoluta: %w", err)
	}

	cmd := exec.Command(
		"npx", "hyperframes", "render",
		absHTML,
		"--output", absVideo,
		"--width", strconv.Itoa(r.cfg.VideoWidth),
		"--height", strconv.Itoa(r.cfg.VideoHeight),
		"--fps", strconv.Itoa(r.cfg.VideoFPS),
		"--duration", "40",
	)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npx hyperframes render falló: %w", err)
	}

	if _, err := os.Stat(absVideo); os.IsNotExist(err) {
		return fmt.Errorf("HyperFrames terminó pero el archivo MP4 no existe: %s", absVideo)
	}
	return nil
}
