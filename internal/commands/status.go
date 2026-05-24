package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
)

func NewStatusCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Muestra el estado del contenido de hoy",
		RunE: func(cmd *cobra.Command, args []string) error {
			today := time.Now().Format("2006-01-02")
			plan, err := database.GetTodayPlan(today)
			if err != nil {
				return fmt.Errorf("consultando plan: %w", err)
			}
			if plan == nil {
				fmt.Printf("No hay contenido planificado para hoy (%s)\n", today)
				return nil
			}

			fmt.Printf("\n📅 Contenido del día: %s\n", today)
			fmt.Printf("   ID:     %d\n", plan.ID)
			fmt.Printf("   Pilar:  %s\n", plan.Pillar)
			fmt.Printf("   Tema:   %s\n", plan.Topic)
			fmt.Printf("   Estado: %s\n", plan.Status)

			if plan.Status == "video_requested" || plan.Status == "rendered" || plan.Status == "pending_approval" {
				job, err := database.GetVideoJobByPlanID(plan.ID)
				if err == nil && job != nil {
					fmt.Printf("   HeyGen: %s (estado: %s)\n", job.HeyGenVideoID, job.Status)
					if job.VideoURL != "" {
						fmt.Printf("   URL:    %s\n", job.VideoURL)
					}
				}
			}
			fmt.Println()
			return nil
		},
	}
}
