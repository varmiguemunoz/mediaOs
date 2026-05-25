package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/evolution"
)

func NewCheckApprovalCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "check-approval",
		Short: "Detecta APPROVE/REJECT por WhatsApp y actualiza estado en DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckApproval(cfg, database)
		},
	}
}

func runCheckApproval(cfg *config.Config, database *db.DB) error {
	if cfg.EvolutionBaseURL == "" || cfg.WhatsAppNumber == "" {
		return fmt.Errorf("EVOLUTION_BASE_URL y WHATSAPP_APPROVAL_NUMBER son requeridos")
	}

	evoClient := evolution.New(cfg)
	since := time.Now().Add(-30 * time.Minute)

	fmt.Println("🔍 Buscando respuestas en los últimos 30 minutos...\n")

	messages, err := evoClient.FindRecentMessagesFrom(cfg.WhatsAppNumber, since)
	if err != nil {
		return fmt.Errorf("consultando mensajes: %w", err)
	}

	if len(messages) == 0 {
		fmt.Println("ℹ️  No hay mensajes nuevos.")
		return nil
	}

	action, planID := evoClient.ExtractApprovalFromMessages(messages)
	if action == "" {
		fmt.Println("ℹ️  No se encontró APPROVE/REJECT en los mensajes recientes.")
		return nil
	}

	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan ID %d no encontrado", planID)
	}

	switch action {
	case "approve":
		job, err := database.GetVideoJobByPlanID(planID)
		if err != nil || job == nil {
			return fmt.Errorf("render job no encontrado para plan %d", planID)
		}
		if err := database.UpdateContentPlanStatus(planID, "approved"); err != nil {
			return fmt.Errorf("actualizando estado: %w", err)
		}
		fmt.Printf("✅ Plan %d aprobado: %s\n", planID, plan.Topic)
		fmt.Printf("   Video listo para publicar manualmente:\n   %s\n", job.VideoURL)

	case "reject":
		if err := database.UpdateContentPlanStatus(planID, "rejected"); err != nil {
			return fmt.Errorf("actualizando estado: %w", err)
		}
		fmt.Printf("❌ Plan %d rechazado: %s\n", planID, plan.Topic)
		fmt.Println("   Puedes regenerar el script con 'content daily'.")
	}

	return nil
}
