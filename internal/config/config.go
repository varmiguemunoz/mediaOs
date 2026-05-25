package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIKey   string
	OpenAIModel string

	HeyGenAPIKey string

	EvolutionBaseURL  string
	EvolutionAPIKey   string
	EvolutionInstance string
	WhatsAppNumber    string

	CronSchedule string

	DBPath string

	HyperFramesDir string
	EditsDir       string
	VideoWidth     int
	VideoHeight    int
	VideoFPS       int
	EditorLayout   string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		OpenAIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:       getEnvOrDefault("OPENAI_MODEL", "gpt-4o"),
		HeyGenAPIKey:      os.Getenv("HEYGEN_API_KEY"),
		EvolutionBaseURL:  os.Getenv("EVOLUTION_BASE_URL"),
		EvolutionAPIKey:   os.Getenv("EVOLUTION_API_KEY"),
		EvolutionInstance: os.Getenv("EVOLUTION_INSTANCE"),
		WhatsAppNumber:    os.Getenv("WHATSAPP_APPROVAL_NUMBER"),
		CronSchedule:      getEnvOrDefault("CRON_SCHEDULE", "0 8 * * 1,4"),
		DBPath:            getEnvOrDefault("DB_PATH", "./content.db"),
		HyperFramesDir:    getEnvOrDefault("HYPERFRAMES_DIR", "./hyperframes"),
		EditsDir:          getEnvOrDefault("EDITS_DIR", "./edits"),
		VideoWidth:        getEnvIntOrDefault("VIDEO_WIDTH", 1080),
		VideoHeight:       getEnvIntOrDefault("VIDEO_HEIGHT", 1920),
		VideoFPS:          getEnvIntOrDefault("VIDEO_FPS", 30),
		EditorLayout:      getEnvOrDefault("EDITOR_LAYOUT", "pip"),
	}

	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY es requerida")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
