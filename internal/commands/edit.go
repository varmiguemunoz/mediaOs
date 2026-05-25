package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/editor"
	"github.com/varmiguemunoz/content-automation/internal/evolution"
	"github.com/varmiguemunoz/content-automation/internal/media"
	"github.com/varmiguemunoz/content-automation/internal/notify"
)

func NewEditCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "edit [plan_id]",
		Short: "Agente 3: combina el video HeyGen + composición HyperFrames con FFmpeg",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("plan_id inválido: %w", err)
			}
			return runEdit(cfg, database, planID)
		},
	}
}

func runEdit(cfg *config.Config, database *db.DB, planID int64) error {
	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan %d no encontrado", planID)
	}

	videoJob, err := database.GetVideoJobByPlanID(planID)
	if err != nil || videoJob == nil {
		return fmt.Errorf("video render job no encontrado para plan %d. ¿Corriste 'content daily'?", planID)
	}
	if videoJob.VideoURL == "" {
		return fmt.Errorf("el video de HeyGen aún no está listo (estado: %s)", videoJob.Status)
	}

	compJob, err := database.GetCompositionJobByPlanID(planID)
	if err != nil || compJob == nil {
		return fmt.Errorf("composition job no encontrado para plan %d. ¿Corriste 'content compose'?", planID)
	}
	if compJob.Status != "composition_ready" {
		return fmt.Errorf("la composición aún no está lista (estado: %s)", compJob.Status)
	}

	fmt.Printf("🎬 AGENTE 3 — Editor FFmpeg\n")
	fmt.Printf("   Plan ID: %d\n", planID)
	fmt.Printf("   Tema: %s\n", plan.Topic)
	fmt.Printf("   Layout: %s\n\n", cfg.EditorLayout)

	outputDir := filepath.Join(cfg.EditsDir, strconv.FormatInt(planID, 10))
	outputPath := filepath.Join(outputDir, "final_video.mp4")

	jobID, err := database.InsertEditJob(db.EditJob{
		ContentPlanID:        planID,
		HeyGenVideoURL:       videoJob.VideoURL,
		CompositionVideoPath: compJob.VideoPath,
	})
	if err != nil {
		return fmt.Errorf("creando edit_job en DB: %w", err)
	}

	comp := editor.New(cfg.EditorLayout)
	editErr := comp.Composite(videoJob.VideoURL, compJob.VideoPath, outputPath, os.Stdout)

	if editErr != nil {
		_ = database.UpdateEditJob(jobID, "failed", "", editErr.Error())
		return fmt.Errorf("edición fallida: %w", editErr)
	}

	if err := database.UpdateEditJob(jobID, "edit_ready", outputPath, ""); err != nil {
		return fmt.Errorf("actualizando edit_job: %w", err)
	}

	if err := database.UpdateContentPlanStatus(planID, "pending_approval"); err != nil {
		fmt.Printf("⚠️  No se pudo actualizar estado del plan: %v\n", err)
	}

	fmt.Printf("\n✅ Video final listo: %s\n", outputPath)

	notify.Desktop("🎬 Video listo", plan.Topic)

	if cfg.EvolutionBaseURL != "" && cfg.WhatsAppNumber != "" {
		script, _ := database.GetScriptByPlanID(planID)
		caption := ""
		if script != nil {
			caption = script.CaptionInstagram
		}
		evoClient := evolution.New(cfg)

		fmt.Println("⏳ Levantando servidor temporal para enviar video...")
		videoURL, serveErr := media.ServeFileTemporarily(outputPath, 5*time.Minute)
		if serveErr != nil {
			fmt.Printf("⚠️  No se pudo servir el video: %v\nEnviando texto como fallback...\n", serveErr)
			msg := buildFinalVideoMessage(planID, plan.Topic, outputPath, caption)
			_ = evoClient.SendText(cfg.WhatsAppNumber, msg)
		} else {
			fmt.Printf("🌐 Video disponible en: %s\n", videoURL)
			if err := evoClient.SendVideo(cfg.WhatsAppNumber, videoURL, fmt.Sprintf("🎬 %s\n\nAPPROVE %d o REJECT %d", plan.Topic, planID, planID)); err != nil {
				fmt.Printf("⚠️  WhatsApp SendVideo: %v\nEnviando texto como fallback...\n", err)
				msg := buildFinalVideoMessage(planID, plan.Topic, videoURL, caption)
				_ = evoClient.SendText(cfg.WhatsAppNumber, msg)
			} else {
				fmt.Println("📱 Video enviado por WhatsApp")
			}
		}
	}

	fmt.Printf("\n💡 Siguiente: APPROVE %d o REJECT %d por WhatsApp\n", planID, planID)
	return nil
}

func buildFinalVideoMessage(planID int64, topic, videoPath, caption string) string {
	msg := fmt.Sprintf("🎬 *Video final listo*\n\n*Tema:* %s\n*Plan ID:* %d\n\n*Archivo:* %s\n\n", topic, planID, videoPath)
	if caption != "" {
		msg += fmt.Sprintf("*Caption Instagram:*\n%s\n\n", caption)
	}
	msg += fmt.Sprintf("Responde:\n✅ APPROVE %d\n❌ REJECT %d", planID, planID)
	return msg
}
