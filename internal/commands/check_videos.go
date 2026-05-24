package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/evolution"
	"github.com/varmiguemunoz/content-automation/internal/heygen"
)

func NewCheckVideosCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "check-videos",
		Short: "Verifica el estado de los renders en HeyGen y notifica por WhatsApp",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckVideos(cfg, database)
		},
	}
}

func runCheckVideos(cfg *config.Config, database *db.DB) error {
	jobs, err := database.ListPendingVideoJobs()
	if err != nil {
		return fmt.Errorf("cargando render jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("ℹ️  No hay videos en estado 'video_requested'.")
		return nil
	}

	fmt.Printf("🔍 Revisando %d render job(s) pendiente(s)...\n\n", len(jobs))

	heygenClient := heygen.New(cfg)
	evoClient := evolution.New(cfg)

	notified := 0
	failed := 0

	for _, job := range jobs {
		status, videoURL, err := heygenClient.GetVideoStatus(job.HeyGenVideoID)
		if err != nil {
			fmt.Printf("  ⚠️  Error revisando %s: %v\n", job.HeyGenVideoID, err)
			continue
		}

		fmt.Printf("  Video %s → estado: %s\n", job.HeyGenVideoID, status)

		switch status {
		case "completed":
			if err := database.UpdateVideoRenderJob(job.ID, "rendered", videoURL, ""); err != nil {
				fmt.Printf("  ⚠️  Error actualizando job: %v\n", err)
				continue
			}

			plan, err := database.GetPlanByID(job.ContentPlanID)
			if err != nil || plan == nil {
				fmt.Printf("  ⚠️  No se encontró el plan %d\n", job.ContentPlanID)
				continue
			}

			if err := database.UpdateContentPlanStatus(plan.ID, "pending_approval"); err != nil {
				fmt.Printf("  ⚠️  Error actualizando estado del plan: %v\n", err)
				continue
			}

			script, err := database.GetScriptByPlanID(plan.ID)
			if err != nil || script == nil {
				fmt.Printf("  ⚠️  Script no encontrado para plan %d\n", plan.ID)
				continue
			}

			if cfg.EvolutionBaseURL == "" || cfg.WhatsAppNumber == "" {
				fmt.Printf("  ℹ️  Video listo pero EvolutionAPI no configurada. URL: %s\n", videoURL)
				continue
			}

			msg := buildApprovalMessage(plan.ID, plan.Topic, videoURL, script.CaptionInstagram)
			if err := evoClient.SendText(cfg.WhatsAppNumber, msg); err != nil {
				fmt.Printf("  ⚠️  Error enviando WhatsApp: %v\n", err)
				continue
			}

			fmt.Printf("  ✅ Video listo y notificación enviada por WhatsApp (plan ID: %d)\n", plan.ID)
			notified++

		case "failed":
			if err := database.UpdateVideoRenderJob(job.ID, "failed", "", "HeyGen reportó fallo en el render"); err != nil {
				fmt.Printf("  ⚠️  Error actualizando job fallido: %v\n", err)
			}
			if err := database.UpdateContentPlanStatus(job.ContentPlanID, "failed"); err != nil {
				fmt.Printf("  ⚠️  Error actualizando estado del plan: %v\n", err)
			}
			fmt.Printf("  ❌ Video fallido (plan ID: %d)\n", job.ContentPlanID)
			failed++

		default:
			fmt.Printf("  ⏳ Aún procesando...\n")
		}
	}

	fmt.Printf("\n✅ Revisión completada: %d notificado(s), %d fallido(s)\n", notified, failed)
	return nil
}

func buildApprovalMessage(planID int64, topic, videoURL, caption string) string {
	var sb strings.Builder
	sb.WriteString("🎬 *Video listo para aprobar*\n\n")
	sb.WriteString(fmt.Sprintf("*Tema:* %s\n", topic))
	sb.WriteString(fmt.Sprintf("*Plan ID:* %d\n\n", planID))
	sb.WriteString(fmt.Sprintf("*URL del video:*\n%s\n\n", videoURL))
	if caption != "" {
		sb.WriteString(fmt.Sprintf("*Caption Instagram:*\n%s\n\n", caption))
	}
	sb.WriteString(fmt.Sprintf("Responde:\n✅ APPROVE %d\n❌ REJECT %d", planID, planID))
	return sb.String()
}
