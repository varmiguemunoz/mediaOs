package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/hyperframes"
	"github.com/varmiguemunoz/content-automation/internal/openai"
)

func NewComposeCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "compose [plan_id]",
		Short: "Agente 2: genera la composición HyperFrames para un plan de contenido",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("plan_id inválido: %w", err)
			}
			return runCompose(cfg, database, planID)
		},
	}
}

func runCompose(cfg *config.Config, database *db.DB, planID int64) error {
	ctx := context.Background()

	plan, err := database.GetPlanByID(planID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan %d no encontrado: %w", planID, err)
	}

	script, err := database.GetScriptByPlanID(planID)
	if err != nil || script == nil {
		return fmt.Errorf("script para plan %d no encontrado. Corre 'content daily' primero", planID)
	}

	if script.TimelineJSON == "" {
		return fmt.Errorf("el script no tiene timeline JSON. Regenera el script con 'content daily'")
	}

	var timeline []openai.TimelineSegment
	if err := json.Unmarshal([]byte(script.TimelineJSON), &timeline); err != nil {
		return fmt.Errorf("parseando timeline JSON: %w", err)
	}
	if len(timeline) == 0 {
		return fmt.Errorf("el timeline está vacío después de deserializar")
	}

	fmt.Printf("🎬 AGENTE 2 — HyperFrames Composition\n")
	fmt.Printf("   Plan ID: %d\n", planID)
	fmt.Printf("   Tema: %s\n", plan.Topic)
	fmt.Printf("   Segmentos de timeline: %d\n\n", len(timeline))

	existingJob, _ := database.GetCompositionJobByPlanID(planID)
	if existingJob != nil && existingJob.Status == "composition_ready" {
		fmt.Printf("ℹ️  Ya existe una composición lista para este plan: %s\n", existingJob.VideoPath)
		fmt.Println("   Usa --force para reemplazarla.")
		return nil
	}

	jobID, err := database.InsertCompositionJob(db.CompositionJob{
		ContentPlanID: planID,
	})
	if err != nil {
		return fmt.Errorf("creando composition_job en DB: %w", err)
	}

	if err := database.UpdateCompositionJob(jobID, "rendering", "", "", ""); err != nil {
		fmt.Printf("⚠️  No se pudo actualizar estado a 'rendering': %v\n", err)
	}

	ai := openai.New(cfg)
	runner := hyperframes.New(cfg, ai)

	htmlPath, videoPath, composeErr := runner.Compose(ctx, hyperframes.CompositionInput{
		PlanID:   planID,
		Topic:    plan.Topic,
		Pillar:   plan.Pillar,
		Timeline: timeline,
		Captions: nil,
	}, os.Stdout)

	if composeErr != nil {
		_ = database.UpdateCompositionJob(jobID, "failed", htmlPath, "", composeErr.Error())
		return fmt.Errorf("composición fallida: %w", composeErr)
	}

	if err := database.UpdateCompositionJob(jobID, "composition_ready", htmlPath, videoPath, ""); err != nil {
		return fmt.Errorf("actualizando job a composition_ready: %w", err)
	}

	fmt.Printf("\n✅ Composición lista\n")
	fmt.Printf("   HTML: %s\n", htmlPath)
	fmt.Printf("   MP4:  %s\n", videoPath)
	fmt.Printf("\n💡 Siguiente paso: cuando HeyGen termine, corre 'content edit %d'\n", planID)
	return nil
}
