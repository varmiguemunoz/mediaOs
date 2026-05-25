package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/evolution"
	"github.com/varmiguemunoz/content-automation/internal/openai"
)

func NewCronCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	var runNow bool

	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Inicia el cron job que genera ideas de contenido 2 veces por semana",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCron(cfg, database, runNow)
		},
	}

	cmd.Flags().BoolVar(&runNow, "now", false, "Ejecuta el job inmediatamente además de programarlo")
	return cmd
}

func runCron(cfg *config.Config, database *db.DB, runNow bool) error {
	fmt.Println("🕐 Iniciando cron de generación de contenido...")
	fmt.Printf("   Plan semanal: %s\n", cfg.CronSchedule)
	fmt.Println("   Check-videos: cada 30 minutos")
	fmt.Println("   Presiona Ctrl+C para detener.\n")

	c := cron.New()

	planFn := func() {
		fmt.Printf("\n[%s] ⚡ Generando plan de contenido...\n", time.Now().Format("2006-01-02 15:04:05"))
		if err := generateWeeklyPlan(cfg, database); err != nil {
			fmt.Printf("[ERROR plan] %v\n", err)
		}
	}

	checkFn := func() {
		fmt.Printf("\n[%s] 🔍 Revisando videos en HeyGen...\n", time.Now().Format("2006-01-02 15:04:05"))
		if err := runCheckVideos(cfg, database); err != nil {
			fmt.Printf("[ERROR check-videos] %v\n", err)
		}
	}

	if _, err := c.AddFunc(cfg.CronSchedule, planFn); err != nil {
		return fmt.Errorf("configurando cron plan: %w", err)
	}

	if _, err := c.AddFunc("*/30 * * * *", checkFn); err != nil {
		return fmt.Errorf("configurando cron check-videos: %w", err)
	}

	c.Start()

	if runNow {
		fmt.Println("▶️  Ejecutando plan ahora por --now flag...\n")
		planFn()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\n⏹  Cron detenido.")
	c.Stop()
	return nil
}

func generateWeeklyPlan(cfg *config.Config, database *db.DB) error {
	ctx := context.Background()
	ai := openai.New(cfg)

	startDate := time.Now().Format("2006-01-02")
	fmt.Printf("📅 Generando 7 ideas desde %s...\n", startDate)

	items, err := ai.GenerateContentPlan(ctx, 7, startDate)
	if err != nil {
		return fmt.Errorf("generando plan: %w", err)
	}

	avatars, err := database.ListActiveAvatars()
	if err != nil {
		return fmt.Errorf("cargando avatares: %w", err)
	}

	savedIDs := []int64{}
	for _, item := range items {
		planID, err := database.InsertContentPlan(db.ContentPlan{
			Date:       item.Date,
			Pillar:     item.Pillar,
			Topic:      item.Topic,
			Angle:      item.Angle,
			Format:     item.Format,
			Difficulty: item.Difficulty,
			CTA:        item.CTA,
		})
		if err != nil {
			fmt.Printf("  ⚠️  Error guardando %s: %v\n", item.Date, err)
			continue
		}

		if len(avatars) > 0 {
			avatar, err := database.GetLeastUsedAvatar()
			if err == nil && avatar != nil {
				_ = database.SetContentPlanAvatar(planID, avatar.ID)
			}
		}

		savedIDs = append(savedIDs, planID)
	}

	fmt.Printf("✅ %d ideas guardadas en DB.\n\n", len(savedIDs))

	if cfg.EvolutionBaseURL != "" && cfg.WhatsAppNumber != "" {
		msg := buildWeeklyPlanMessage(items)
		evoClient := evolution.New(cfg)
		if err := evoClient.SendText(cfg.WhatsAppNumber, msg); err != nil {
			fmt.Printf("⚠️  WhatsApp no enviado: %v\n", err)
		} else {
			fmt.Println("📲 Resumen enviado por WhatsApp.")
		}
	} else {
		fmt.Println("ℹ️  EvolutionAPI no configurado, resumen solo en terminal:")
		fmt.Println(buildWeeklyPlanMessage(items))
	}

	return nil
}

func buildWeeklyPlanMessage(items []openai.ContentPlanItem) string {
	var sb strings.Builder
	sb.WriteString("🗓️ *Plan de contenido semanal*\n\n")

	for _, item := range items {
		sb.WriteString(fmt.Sprintf("📅 *%s*\n", item.Date))
		sb.WriteString(fmt.Sprintf("   Pilar: %s\n", item.Pillar))
		sb.WriteString(fmt.Sprintf("   Tema: %s\n", item.Topic))
		sb.WriteString(fmt.Sprintf("   Ángulo: %s\n", item.Angle))
		sb.WriteString(fmt.Sprintf("   Dificultad: %s\n", item.Difficulty))
		sb.WriteString(fmt.Sprintf("   CTA: %s\n\n", item.CTA))
	}

	sb.WriteString("Corre 'content daily' cada mañana para generar el script del día.")
	return sb.String()
}
