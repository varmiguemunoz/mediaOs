package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/heygen"
	"github.com/varmiguemunoz/content-automation/internal/openai"
)

func NewDailyCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "daily",
		Short: "Genera el script del día y crea el video en HeyGen",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaily(cfg, database)
		},
	}
}

func runDaily(cfg *config.Config, database *db.DB) error {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	fmt.Printf("📅 Ejecutando daily runner para: %s\n\n", today)

	plan, err := database.GetTodayPlan(today)
	if err != nil {
		return fmt.Errorf("consultando plan: %w", err)
	}
	if plan == nil {
		fmt.Printf("ℹ️  No hay contenido planificado para hoy (%s).\n", today)
		fmt.Println("   Corre 'content plan' para generar el plan de la semana.")
		return nil
	}

	fmt.Printf("📌 Tema: %s\n", plan.Topic)
	fmt.Printf("   Pilar: %s | Dificultad: %s\n", plan.Pillar, plan.Difficulty)
	fmt.Printf("   Ángulo: %s\n", plan.Angle)
	fmt.Printf("   CTA: %s\n\n", plan.CTA)

	avatar, err := database.GetAvatarByID(*plan.AvatarID)
	if err != nil || avatar == nil {
		return fmt.Errorf("avatar no encontrado (ID: %d)", *plan.AvatarID)
	}
	if avatar.HeyGenVoiceID == "" {
		return fmt.Errorf("el avatar '%s' no tiene heygen_voice_id. Actualízalo en la DB antes de continuar", avatar.Name)
	}

	fmt.Printf("🎭 Avatar: %s\n\n", avatar.Name)

	ai := openai.New(cfg)
	fmt.Println("⏳ Generando script con OpenAI...")

	script, err := ai.GenerateVideoScript(ctx, plan.Topic, plan.Pillar, plan.Angle, plan.CTA)
	if err != nil {
		return fmt.Errorf("generando script: %w", err)
	}

	wordCount := len(strings.Fields(script.Script))
	fmt.Printf("✅ Script generado (%d palabras)\n\n", wordCount)

	scriptID, err := database.InsertGeneratedScript(db.GeneratedScript{
		ContentPlanID:            plan.ID,
		Title:                    script.Title,
		Script:                   script.Script,
		CaptionInstagram:         script.CaptionInstagram,
		CaptionFacebook:          script.CaptionFacebook,
		CaptionLinkedIn:          script.CaptionLinkedIn,
		CaptionTikTok:            script.CaptionTikTok,
		Hashtags:                 strings.Join(script.Hashtags, " "),
		CTA:                      script.CTA,
		EstimatedDurationSeconds: script.EstimatedDurationSeconds,
		WordCount:                wordCount,
	})
	if err != nil {
		return fmt.Errorf("guardando script: %w", err)
	}
	_ = scriptID

	if err := database.UpdateContentPlanStatus(plan.ID, "script_generated"); err != nil {
		return fmt.Errorf("actualizando estado: %w", err)
	}

	heygenClient := heygen.New(cfg)
	fmt.Println("⏳ Enviando a HeyGen...")

	videoID, err := heygenClient.CreateVideo(avatar.HeyGenAvatarID, avatar.HeyGenVoiceID, script.Script)
	if err != nil {
		return fmt.Errorf("creando video en HeyGen: %w", err)
	}

	_, err = database.InsertVideoRenderJob(db.VideoRenderJob{
		ContentPlanID: plan.ID,
		HeyGenVideoID: videoID,
	})
	if err != nil {
		return fmt.Errorf("guardando render job: %w", err)
	}

	if err := database.UpdateAvatarLastUsed(avatar.ID); err != nil {
		fmt.Printf("⚠️  No se pudo actualizar last_used_at del avatar: %v\n", err)
	}

	if err := database.UpdateContentPlanStatus(plan.ID, "video_requested"); err != nil {
		return fmt.Errorf("actualizando estado: %w", err)
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🎬 RESUMEN DEL DÍA")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Tema:        %s\n", plan.Topic)
	fmt.Printf("Avatar:      %s\n", avatar.Name)
	fmt.Printf("Script:      %d palabras\n", wordCount)
	fmt.Printf("HeyGen ID:   %s\n", videoID)
	fmt.Printf("Estado:      video_requested\n")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("\n📜 Script:")
	fmt.Println()
	fmt.Println(script.Script)
	fmt.Println()
	fmt.Println("⏳ El video está renderizando. Corre 'content check-videos' para revisar el estado.")
	return nil
}
