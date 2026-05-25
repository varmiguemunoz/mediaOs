package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Configura el proyecto y lo instala como servicio de sistema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWizard()
		},
	}
}

func runInitWizard() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("el comando 'init' solo está disponible en macOS")
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║       mediaOS — Setup Wizard           ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	if err := ensureHomebrew(); err != nil {
		fmt.Printf("⚠️  Homebrew: %v\n", err)
	}
	if err := ensureBrewPackage("node@22", "node"); err != nil {
		fmt.Printf("⚠️  Node 22: %v\n", err)
	}
	if err := ensureBrewPackage("ffmpeg", "ffmpeg"); err != nil {
		fmt.Printf("⚠️  FFmpeg: %v\n", err)
	}
	if err := ensureHyperFrames(); err != nil {
		fmt.Printf("⚠️  HyperFrames: %v\n", err)
	}

	fmt.Println()
	fmt.Println("── Configuración de API Keys ──────────────")
	fmt.Println()

	existing := loadExistingEnv(".env")

	type field struct {
		key      string
		prompt   string
		fallback string
	}
	fields := []field{
		{"OPENAI_API_KEY", "OpenAI API Key (sk-...)", ""},
		{"OPENAI_MODEL", "OpenAI Model", "gpt-4o"},
		{"HEYGEN_API_KEY", "HeyGen API Key", ""},
		{"EVOLUTION_BASE_URL", "Evolution API Base URL (ej: https://tudominio.com)", ""},
		{"EVOLUTION_API_KEY", "Evolution API Key", ""},
		{"EVOLUTION_INSTANCE", "Evolution Instance name", ""},
		{"WHATSAPP_APPROVAL_NUMBER", "Tu número de WhatsApp (ej: 573001234567)", ""},
		{"CRON_SCHEDULE", "Cron schedule", "0 8 * * 1,4"},
		{"DB_PATH", "Ruta de la base de datos", "./content.db"},
		{"HYPERFRAMES_DIR", "Directorio HyperFrames", "./hyperframes"},
		{"EDITS_DIR", "Directorio de ediciones", "./edits"},
		{"VIDEO_WIDTH", "Ancho del video", "1080"},
		{"VIDEO_HEIGHT", "Alto del video", "1920"},
		{"VIDEO_FPS", "FPS del video", "30"},
		{"EDITOR_LAYOUT", "Layout del editor (pip/split)", "pip"},
	}

	reader := bufio.NewReader(os.Stdin)
	cfg := make(map[string]string)
	asked := 0

	for _, f := range fields {
		if val, ok := existing[f.key]; ok && val != "" {
			cfg[f.key] = val
			fmt.Printf("✅ %-30s ya configurada\n", f.key)
			continue
		}
		effective := f.fallback
		if existing[f.key] != "" {
			effective = existing[f.key]
		}
		if effective != "" {
			fmt.Printf("%s [%s]: ", f.prompt, effective)
		} else {
			fmt.Printf("%s: ", f.prompt)
		}
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			cfg[f.key] = effective
		} else {
			cfg[f.key] = line
		}
		asked++
	}

	if asked == 0 {
		fmt.Println("\n✅ Todas las variables ya estaban configuradas en .env")
	}

	if cfg["OPENAI_API_KEY"] == "" {
		return fmt.Errorf("OPENAI_API_KEY es requerida")
	}

	fmt.Println()
	fmt.Println("── Escribiendo .env ────────────────────────")
	if err := writeEnvFile(cfg); err != nil {
		return fmt.Errorf("escribiendo .env: %w", err)
	}
	fmt.Println("✅ .env creado")

	fmt.Println()
	fmt.Println("── Compilando binario ──────────────────────")
	if err := buildBinary(); err != nil {
		return fmt.Errorf("compilando: %w", err)
	}
	fmt.Println("✅ Binario compilado en ./bin/content")

	fmt.Println()
	fmt.Println("── Instalando servicio (LaunchAgent) ───────")
	if err := installLaunchAgent(); err != nil {
		return fmt.Errorf("instalando LaunchAgent: %w", err)
	}
	fmt.Println("✅ Servicio registrado")

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║   ¡Instalación completa! 🚀             ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("El cron corre automáticamente según tu schedule.")
	fmt.Println("Logs en: ./logs/content.log")
	fmt.Println()
	fmt.Println("Comandos útiles:")
	fmt.Println("  make run-plan          → genera el plan de contenido")
	fmt.Println("  make run-daily         → corre el pipeline hoy")
	fmt.Println("  make run-check-videos  → revisa renders HeyGen")
	fmt.Println("  content uninstall      → elimina el servicio del sistema")
	fmt.Println()
	return nil
}

func loadExistingEnv(path string) map[string]string {
	result := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func ensureHomebrew() error {
	if _, err := exec.LookPath("brew"); err == nil {
		fmt.Println("✅ Homebrew ya instalado")
		return nil
	}
	fmt.Println("⏳ Instalando Homebrew...")
	cmd := exec.Command("/bin/bash", "-c",
		`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureBrewPackage(brewName, binaryName string) error {
	if _, err := exec.LookPath(binaryName); err == nil {
		fmt.Printf("✅ %s ya instalado\n", brewName)
		return nil
	}
	fmt.Printf("⏳ Instalando %s...\n", brewName)
	cmd := exec.Command("brew", "install", brewName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install %s: %w", brewName, err)
	}
	fmt.Printf("✅ %s instalado\n", brewName)
	return nil
}

func ensureHyperFrames() error {
	if out, err := exec.Command("npx", "hyperframes", "--version").Output(); err == nil {
		fmt.Printf("✅ HyperFrames ya instalado (%s)\n", strings.TrimSpace(string(out)))
		return nil
	}
	fmt.Println("⏳ Instalando hyperframes via npm...")
	cmd := exec.Command("npm", "install", "-g", "hyperframes")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install -g hyperframes: %w", err)
	}
	fmt.Println("✅ HyperFrames instalado")
	return nil
}

func writeEnvFile(cfg map[string]string) error {
	var sb strings.Builder
	keys := []string{
		"OPENAI_API_KEY", "OPENAI_MODEL",
		"HEYGEN_API_KEY",
		"EVOLUTION_BASE_URL", "EVOLUTION_API_KEY", "EVOLUTION_INSTANCE", "WHATSAPP_APPROVAL_NUMBER",
		"CRON_SCHEDULE", "DB_PATH",
		"HYPERFRAMES_DIR", "VIDEO_WIDTH", "VIDEO_HEIGHT", "VIDEO_FPS",
		"EDITS_DIR", "EDITOR_LAYOUT",
	}
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, cfg[k]))
	}
	return os.WriteFile(".env", []byte(sb.String()), 0600)
}

func buildBinary() error {
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", "bin/content", "./cmd/content")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mediaos.content</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>cron</string>
    </array>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.ErrLogPath}}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>{{.Home}}</string>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
`

func installLaunchAgent() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("obteniendo ruta del binario: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}

	home := os.Getenv("HOME")
	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return err
	}

	plistPath := filepath.Join(launchAgentsDir, "com.mediaos.content.plist")

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("creando plist: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, map[string]string{
		"BinaryPath": exe,
		"WorkDir":    workDir,
		"LogPath":    filepath.Join(workDir, "logs", "content.log"),
		"ErrLogPath": filepath.Join(workDir, "logs", "content.error.log"),
		"Home":       home,
	}); err != nil {
		return err
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()
	cmd := exec.Command("launchctl", "load", plistPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	fmt.Printf("   Plist: %s\n", plistPath)
	return nil
}
