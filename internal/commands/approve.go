package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/services"
)

func NewApproveCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "approve [plan-id]",
		Short: "Aprueba manualmente un plan y publica el contenido",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("ID inválido: %s", args[0])
			}
			return runApprove(cfg, database, id)
		},
	}
}

func runApprove(cfg *config.Config, database *db.DB, planID int64) error {
	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan ID %d no encontrado", planID)
	}

	job, err := database.GetVideoJobByPlanID(planID)
	if err != nil || job == nil {
		return fmt.Errorf("render job no encontrado para plan %d. ¿Ya corriste 'content daily'?", planID)
	}

	if job.VideoURL == "" {
		return fmt.Errorf("el video aún no tiene URL. Estado actual: %s. Corre 'content check-videos' primero", job.Status)
	}

	script, err := database.GetScriptByPlanID(planID)
	if err != nil || script == nil {
		return fmt.Errorf("script no encontrado para plan %d", planID)
	}

	fmt.Printf("📌 Aprobando plan %d: %s\n\n", planID, plan.Topic)

	publisher := services.NewPublisher(cfg, database)
	if err := publisher.Publish(plan, script, job.VideoURL); err != nil {
		return fmt.Errorf("publicando: %w", err)
	}

	if err := database.UpdateContentPlanStatus(planID, "published"); err != nil {
		return fmt.Errorf("actualizando estado: %w", err)
	}

	fmt.Printf("\n🎉 Plan %d publicado exitosamente.\n", planID)
	return nil
}
