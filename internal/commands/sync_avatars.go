package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
	"github.com/varmiguemunoz/content-automation/internal/heygen"
)

func NewSyncAvatarsCmd(cfg *config.Config, database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "sync-avatars",
		Short: "Sincroniza avatares desde HeyGen a la base de datos",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncAvatars(cfg, database)
		},
	}
}

func runSyncAvatars(cfg *config.Config, database *db.DB) error {
	if cfg.HeyGenAPIKey == "" {
		return fmt.Errorf("HEYGEN_API_KEY no está configurada")
	}

	client := heygen.New(cfg)
	fmt.Println("⏳ Conectando a HeyGen...")

	groups, err := client.ListAvatarGroups()
	if err != nil {
		return fmt.Errorf("listando avatares de HeyGen: %w", err)
	}

	if len(groups) == 0 {
		fmt.Println("⚠️  No se encontraron grupos de avatares en tu cuenta de HeyGen.")
		fmt.Println("   Asegúrate de tener avatares creados en app.heygen.com")
		return nil
	}

	synced := 0
	for _, group := range groups {
		for _, avatar := range group.Avatars {
			a := db.Avatar{
				Name:           group.AvatarName,
				HeyGenAvatarID: avatar.AvatarID,
				HeyGenVoiceID:  "",
				Style:          "",
				BestFor:        "",
				Active:         true,
			}
			if err := database.UpsertAvatar(a); err != nil {
				fmt.Printf("   ⚠️  Error sincronizando %s: %v\n", group.AvatarName, err)
				continue
			}
			synced++
			fmt.Printf("   ✓ %s (ID: %s)\n", group.AvatarName, avatar.AvatarID)
		}
	}

	fmt.Printf("\n✅ %d avatares sincronizados.\n", synced)
	fmt.Println()
	fmt.Println("IMPORTANTE: Los avatares se guardaron sin voice_id.")
	fmt.Println("Debes actualizar manualmente el heygen_voice_id de cada avatar en la DB.")
	fmt.Println("Para ver tus voice IDs disponibles: https://app.heygen.com/voices")
	fmt.Println()
	fmt.Println("Ejemplo de update directo en SQLite:")
	fmt.Println("  sqlite3 content.db \"UPDATE avatars SET heygen_voice_id='voz_xxx' WHERE name='TuNombre'\"")

	return nil
}
