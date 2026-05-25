package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/varmiguemunoz/content-automation/internal/commands"
	"github.com/varmiguemunoz/content-automation/internal/config"
	"github.com/varmiguemunoz/content-automation/internal/db"
)

func main() {
	root := &cobra.Command{
		Use:   "content",
		Short: "Sistema de generación de contenido automatizado",
	}

	root.AddCommand(commands.NewInitCmd())
	root.AddCommand(commands.NewUninstallCmd())

	cfg, err := config.Load()
	if err != nil {
		if isInitOrUninstall() {
			if err := root.Execute(); err != nil {
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Tip: corre 'content init' para configurar el proyecto.\n")
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	root.AddCommand(
		commands.NewCronCmd(cfg, database),
		commands.NewPlanCmd(cfg, database),
		commands.NewDailyCmd(cfg, database),
		commands.NewSyncAvatarsCmd(cfg, database),
		commands.NewCheckVideosCmd(cfg, database),
		commands.NewCheckApprovalCmd(cfg, database),
		commands.NewApproveCmd(cfg, database),
		commands.NewStatusCmd(cfg, database),
		commands.NewComposeCmd(cfg, database),
		commands.NewEditCmd(cfg, database),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func isInitOrUninstall() bool {
	for _, arg := range os.Args[1:] {
		if arg == "init" || arg == "uninstall" {
			return true
		}
	}
	return false
}
