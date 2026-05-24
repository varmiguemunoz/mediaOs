package config

import (
	"fmt"
	"os"

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

	MetaPageID          string
	MetaPageAccessToken string
	MetaIGUserID        string

	TikTokAccessToken string
	TikTokOpenID      string

	DBPath string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		OpenAIKey:           os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:         getEnvOrDefault("OPENAI_MODEL", "gpt-4o"),
		HeyGenAPIKey:        os.Getenv("HEYGEN_API_KEY"),
		EvolutionBaseURL:    os.Getenv("EVOLUTION_BASE_URL"),
		EvolutionAPIKey:     os.Getenv("EVOLUTION_API_KEY"),
		EvolutionInstance:   os.Getenv("EVOLUTION_INSTANCE"),
		WhatsAppNumber:      os.Getenv("WHATSAPP_APPROVAL_NUMBER"),
		MetaPageID:          os.Getenv("META_PAGE_ID"),
		MetaPageAccessToken: os.Getenv("META_PAGE_ACCESS_TOKEN"),
		MetaIGUserID:        os.Getenv("META_IG_USER_ID"),
		TikTokAccessToken:   os.Getenv("TIKTOK_ACCESS_TOKEN"),
		TikTokOpenID:        os.Getenv("TIKTOK_OPEN_ID"),
		DBPath:              getEnvOrDefault("DB_PATH", "./content.db"),
	}

	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
