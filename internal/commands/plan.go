package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/openai"
)

func NewPlanCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Genera el plan de contenido y asigna avatares",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cfg, database, days)
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Número de días a planificar")
	return cmd
}

func runPlan(cfg *config.Config, database *db.DB, days int) error {
	ctx := context.Background()
	ai := openai.New(cfg)

	avatars, err := database.ListActiveAvatars()
	if err != nil {
		return fmt.Errorf("cargando avatares: %w", err)
	}
	if len(avatars) == 0 {
		return fmt.Errorf("no hay avatares en la base de datos. Corre primero: content sync-avatars")
	}

	startDate := time.Now().Format("2006-01-02")
	fmt.Printf("⏳ Generando plan de %d días desde %s...\n", days, startDate)

	items, err := ai.GenerateContentPlan(ctx, days, startDate)
	if err != nil {
		return fmt.Errorf("generando plan: %w", err)
	}

	fmt.Printf("\n✅ Plan generado (%d ítems):\n\n", len(items))
	printPlanSummary(items)

	fmt.Println("\n🎭 Avatares disponibles:")
	printAvatarList(avatars)

	scanner := bufio.NewScanner(os.Stdin)
	savedIDs := []int64{}

	for _, item := range items {
		planID, err := database.InsertContentPlan(db.ContentPlan{
			Date:       item.Date,
			Pillar:     item.Pillar,
			Topic:      item.Topic,
			Angle:      item.Angle,
			Format:     item.Format,
			Difficulty: item.Difficulty,
			CTA:        item.CTA,
		})
		if err != nil {
			return fmt.Errorf("guardando plan %s: %w", item.Date, err)
		}
		savedIDs = append(savedIDs, planID)
	}

	fmt.Println("Asigna un avatar a cada día (escribe el número):")
	for i, item := range items {
		planID := savedIDs[i]

		fmt.Printf("\n📅 %s — %s\n", item.Date, item.Topic)
		fmt.Printf("   Pilar: %s | Dificultad: %s\n", item.Pillar, item.Difficulty)
		fmt.Print("   Avatar (número): ")

		avatarID := int64(0)
		for avatarID == 0 {
			if !scanner.Scan() {
				return fmt.Errorf("error leyendo entrada")
			}
			input := strings.TrimSpace(scanner.Text())
			n, err := strconv.Atoi(input)
			if err != nil || n < 1 || n > len(avatars) {
				fmt.Printf("   ⚠️  Ingresa un número entre 1 y %d: ", len(avatars))
				continue
			}
			avatarID = avatars[n-1].ID
		}

		if err := database.SetContentPlanAvatar(planID, avatarID); err != nil {
			return fmt.Errorf("asignando avatar: %w", err)
		}

		chosenAvatar := avatars[indexOfAvatar(avatars, avatarID)]
		fmt.Printf("   ✓ Avatar asignado: %s\n", chosenAvatar.Name)
	}

	fmt.Printf("\n🎉 Plan guardado. %d días de contenido listos.\n", days)
	fmt.Println("   Corre 'content daily' cada día para generar el script y enviarlo a HeyGen.")
	return nil
}

func printAvatarList(avatars []db.Avatar) {
	fmt.Println()
	for i, a := range avatars {
		lastUsed := "nunca usado"
		if a.LastUsedAt != nil {
			lastUsed = "usado: " + a.LastUsedAt.Format("2006-01-02")
		}
		style := a.Style
		if style == "" {
			style = "sin estilo definido"
		}
		fmt.Printf("  [%d] %s — %s (%s)\n", i+1, a.Name, style, lastUsed)
	}
	fmt.Println()
}

func indexOfAvatar(avatars []db.Avatar, id int64) int {
	for i, a := range avatars {
		if a.ID == id {
			return i
		}
	}
	return 0
}

func printPlanSummary(items []openai.ContentPlanItem) {
	fmt.Printf("  %-12s %-28s %-26s %s\n", "Fecha", "Tema", "Pilar", "Dificultad")
	fmt.Println("  " + strings.Repeat("─", 78))
	for _, item := range items {
		topic := truncate(item.Topic, 27)
		pillar := truncate(item.Pillar, 25)
		fmt.Printf("  %-12s %-28s %-26s %s\n", item.Date, topic, pillar, item.Difficulty)
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
