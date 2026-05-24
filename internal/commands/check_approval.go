package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/evolution"
	"github.com/varmiguemunoz/content-automation/internal/services"
)

func NewCheckApprovalCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "check-approval",
		Short: "Verifica respuestas de aprobación en WhatsApp y publica si fue aprobado",
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

	fmt.Printf("🔍 Buscando respuestas de aprobación en los últimos 30 minutos...\n\n")

	messages, err := evoClient.FindRecentMessagesFrom(cfg.WhatsAppNumber, since)
	if err != nil {
		return fmt.Errorf("consultando mensajes: %w", err)
	}

	if len(messages) == 0 {
		fmt.Println("ℹ️  No hay mensajes nuevos de aprobación.")
		return nil
	}

	action, planID := evoClient.ExtractApprovalFromMessages(messages)
	if action == "" {
		fmt.Println("ℹ️  No se encontró ninguna instrucción APPROVE/REJECT en los mensajes recientes.")
		return nil
	}

	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan ID %d no encontrado", planID)
	}

	if plan.Status != "pending_approval" {
		fmt.Printf("ℹ️  El plan %d está en estado '%s', no en 'pending_approval'. Ignorando.\n", planID, plan.Status)
		return nil
	}

	switch action {
	case "approve":
		fmt.Printf("✅ Aprobado: plan %d — %s\n\n", planID, plan.Topic)

		job, err := database.GetVideoJobByPlanID(planID)
		if err != nil || job == nil {
			return fmt.Errorf("render job no encontrado para plan %d", planID)
		}

		script, err := database.GetScriptByPlanID(planID)
		if err != nil || script == nil {
			return fmt.Errorf("script no encontrado para plan %d", planID)
		}

		publisher := services.NewPublisher(cfg, database)
		if err := publisher.Publish(plan, script, job.VideoURL); err != nil {
			return fmt.Errorf("publicando: %w", err)
		}

		if err := database.UpdateContentPlanStatus(planID, "published"); err != nil {
			return fmt.Errorf("actualizando estado: %w", err)
		}

		fmt.Printf("\n🎉 Publicado exitosamente en todas las plataformas.\n")

	case "reject":
		fmt.Printf("❌ Rechazado: plan %d — %s\n", planID, plan.Topic)
		if err := database.UpdateContentPlanStatus(planID, "rejected"); err != nil {
			return fmt.Errorf("actualizando estado: %w", err)
		}
		fmt.Println("   Estado actualizado a 'rejected'. Puedes regenerar el script con 'content daily'.")
	}

	return nil
}
