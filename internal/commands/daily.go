package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/heygen"
	"github.com/varmiguemunoz/content-automation/internal/hyperframes"
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
	fmt.Printf("✅ Script generado (%d palabras, %d segmentos de timeline)\n\n", wordCount, len(script.Timeline))

	timelineJSON := ""
	if len(script.Timeline) > 0 {
		raw, err := json.Marshal(script.Timeline)
		if err != nil {
			fmt.Printf("⚠️  No se pudo serializar el timeline: %v\n", err)
		} else {
			timelineJSON = string(raw)
		}
	}

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
		TimelineJSON:             timelineJSON,
	})
	if err != nil {
		return fmt.Errorf("guardando script: %w", err)
	}
	_ = scriptID

	if err := database.UpdateContentPlanStatus(plan.ID, "script_generated"); err != nil {
		return fmt.Errorf("actualizando estado: %w", err)
	}

	var (
		wg          sync.WaitGroup
		heygenID    string
		heygenErr   error
		composeErr  error
		compJobID   int64
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		fmt.Println("🤖 [Agente 1] Enviando a HeyGen...")
		hc := heygen.New(cfg)
		heygenID, heygenErr = hc.CreateVideo(avatar.HeyGenAvatarID, avatar.HeyGenVoiceID, script.Script)
		if heygenErr != nil {
			fmt.Printf("❌ [Agente 1] HeyGen error: %v\n", heygenErr)
			return
		}
		if _, err := database.InsertVideoRenderJob(db.VideoRenderJob{
			ContentPlanID: plan.ID,
			HeyGenVideoID: heygenID,
		}); err != nil {
			heygenErr = fmt.Errorf("guardando render job: %w", err)
			return
		}
		_ = database.UpdateAvatarLastUsed(avatar.ID)
		_ = database.UpdateContentPlanStatus(plan.ID, "video_requested")
		fmt.Printf("✅ [Agente 1] HeyGen video en cola: %s\n", heygenID)
	}()

	go func() {
		defer wg.Done()
		if timelineJSON == "" || len(script.Timeline) == 0 {
			fmt.Println("⚠️  [Agente 2] Sin timeline, saltando HyperFrames")
			return
		}
		fmt.Println("🤖 [Agente 2] Iniciando composición HyperFrames...")
		var jobIDErr error
		compJobID, jobIDErr = database.InsertCompositionJob(db.CompositionJob{ContentPlanID: plan.ID})
		if jobIDErr != nil {
			composeErr = fmt.Errorf("creando composition_job: %w", jobIDErr)
			return
		}
		_ = database.UpdateCompositionJob(compJobID, "rendering", "", "", "")

		ai := openai.New(cfg)
		runner := hyperframes.New(cfg, ai)
		htmlPath, videoPath, err := runner.Compose(ctx, hyperframes.CompositionInput{
			PlanID:   plan.ID,
			Topic:    plan.Topic,
			Pillar:   plan.Pillar,
			Timeline: script.Timeline,
			Captions: script.Captions,
		}, os.Stdout)

		if err != nil {
			composeErr = err
			_ = database.UpdateCompositionJob(compJobID, "failed", htmlPath, "", err.Error())
			fmt.Printf("❌ [Agente 2] HyperFrames error: %v\n", err)
			return
		}
		_ = database.UpdateCompositionJob(compJobID, "composition_ready", htmlPath, videoPath, "")
		fmt.Printf("✅ [Agente 2] Composición lista: %s\n", videoPath)
	}()

	wg.Wait()

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🎬 RESUMEN DEL DÍA")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Tema:        %s\n", plan.Topic)
	fmt.Printf("Avatar:      %s\n", avatar.Name)
	fmt.Printf("Script:      %d palabras\n", wordCount)
	if heygenErr == nil {
		fmt.Printf("HeyGen ID:   %s ✅\n", heygenID)
	} else {
		fmt.Printf("HeyGen:      ❌ %v\n", heygenErr)
	}
	if composeErr == nil && compJobID > 0 {
		fmt.Printf("Composición: ✅ job #%d\n", compJobID)
	} else if composeErr != nil {
		fmt.Printf("Composición: ❌ %v\n", composeErr)
	} else {
		fmt.Println("Composición: ⏭ saltada (sin timeline)")
	}
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("\n📜 Script:")
	fmt.Println()
	fmt.Println(script.Script)
	fmt.Println()
	fmt.Println("⏳ Corre 'content check-videos' para detectar cuando HeyGen termine y lanzar el editor.")
	return nil
}
