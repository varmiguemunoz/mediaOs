package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/editor"
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

			script, _ := database.GetScriptByPlanID(plan.ID)

			compJob, _ := database.GetCompositionJobByPlanID(plan.ID)
			if compJob != nil && compJob.Status == "composition_ready" {
				fmt.Printf("  🎬 Composición lista — lanzando Agente 3 (FFmpeg)...\n")
				outputPath := filepath.Join(cfg.EditsDir, strconv.FormatInt(plan.ID, 10), "final_video.mp4")
				jobID, _ := database.InsertEditJob(db.EditJob{
					ContentPlanID:        plan.ID,
					HeyGenVideoURL:       videoURL,
					CompositionVideoPath: compJob.VideoPath,
				})
				comp := editor.New(cfg.EditorLayout)
				editErr := comp.Composite(videoURL, compJob.VideoPath, outputPath, os.Stdout)
				if editErr != nil {
					_ = database.UpdateEditJob(jobID, "failed", "", editErr.Error())
					fmt.Printf("  ❌ FFmpeg falló: %v\n", editErr)
				} else {
					_ = database.UpdateEditJob(jobID, "edit_ready", outputPath, "")
					_ = database.UpdateContentPlanStatus(plan.ID, "pending_approval")
					fmt.Printf("  ✅ Video final listo: %s\n", outputPath)

					if cfg.EvolutionBaseURL != "" && cfg.WhatsAppNumber != "" {
						caption := ""
						if script != nil {
							caption = script.CaptionInstagram
						}
						msg := buildFinalVideoMessage(plan.ID, plan.Topic, outputPath, caption)
						if err := evoClient.SendText(cfg.WhatsAppNumber, msg); err != nil {
							fmt.Printf("  ⚠️  WhatsApp: %v\n", err)
						} else {
							notified++
						}
					}
				}
			} else {
				_ = database.UpdateContentPlanStatus(plan.ID, "pending_approval")
				fmt.Printf("  ℹ️  HeyGen listo pero sin composición. Enviando URL de HeyGen directamente.\n")
				if cfg.EvolutionBaseURL != "" && cfg.WhatsAppNumber != "" {
					caption := ""
					if script != nil {
						caption = script.CaptionInstagram
					}
					msg := buildApprovalMessage(plan.ID, plan.Topic, videoURL, caption)
					if err := evoClient.SendText(cfg.WhatsAppNumber, msg); err != nil {
						fmt.Printf("  ⚠️  WhatsApp: %v\n", err)
					} else {
						notified++
					}
				}
			}
			fmt.Printf("  ✅ Plan %d procesado\n", plan.ID)

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
