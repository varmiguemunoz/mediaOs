package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
)

func NewApproveCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "approve [plan-id]",
		Short: "Marca un plan como aprobado para publicación manual",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("ID inválido: %s", args[0])
			}
			return runApprove(database, id)
		},
	}
}

func runApprove(database *db.DB, planID int64) error {
	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan ID %d no encontrado", planID)
	}

	job, err := database.GetVideoJobByPlanID(planID)
	if err != nil || job == nil {
		return fmt.Errorf("render job no encontrado para plan %d. ¿Ya corriste 'content daily'?", planID)
	}

	if job.VideoURL == "" {
		return fmt.Errorf("el video aún no tiene URL (estado: %s). Corre 'content check-videos' primero", job.Status)
	}

	fmt.Printf("📌 Plan %d aprobado: %s\n", planID, plan.Topic)
	fmt.Printf("   URL del video: %s\n\n", job.VideoURL)

	if err := database.UpdateContentPlanStatus(planID, "approved"); err != nil {
		return fmt.Errorf("actualizando estado: %w", err)
	}

	fmt.Println("✅ Estado actualizado a 'approved'.")
	fmt.Println("   Descarga el video y publícalo manualmente en tus redes.")
	return nil
}
