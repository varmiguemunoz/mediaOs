package editor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type Compositor struct {
	layout string
}

func New(layout string) *Compositor {
	return &Compositor{layout: layout}
}

func (c *Compositor) Composite(heygenVideoURL, compositionPath, outputPath string, logWriter io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creando directorio de salida: %w", err)
	}

	heygenLocal := outputPath + ".heygen_tmp.mp4"
	fmt.Fprintf(logWriter, "⏳ Descargando video de HeyGen...\n")
	if err := downloadFile(heygenVideoURL, heygenLocal); err != nil {
		return fmt.Errorf("descargando video HeyGen: %w", err)
	}
	defer os.Remove(heygenLocal)
	fmt.Fprintf(logWriter, "✅ Video descargado\n")

	fmt.Fprintf(logWriter, "⏳ Ejecutando FFmpeg (%s)...\n", c.layout)
	if err := c.runFFmpeg(heygenLocal, compositionPath, outputPath, logWriter); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("FFmpeg terminó pero el archivo final no existe: %s", outputPath)
	}
	return nil
}

func (c *Compositor) runFFmpeg(heygenPath, compositionPath, outputPath string, logWriter io.Writer) error {
	var filterComplex string
	switch c.layout {
	case "split":
		filterComplex = "[0:v]scale=540:-1[left];[1:v]scale=540:-1[right];[left][right]hstack[v]"
	default:
		pipScale := "0.3"
		filterComplex = fmt.Sprintf("[1:v]scale=iw*%s:-1[pip];[0:v][pip]overlay=W-w-20:H-h-20[v]", pipScale)
	}

	pipScaleFloat := 0.3
	_ = pipScaleFloat

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", compositionPath,
		"-i", heygenPath,
		"-filter_complex", filterComplex,
		"-map", "[v]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-shortest",
		outputPath,
	)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	return cmd.Run()
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", strconv.Itoa(resp.StatusCode))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
