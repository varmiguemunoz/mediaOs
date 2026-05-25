package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Elimina el servicio de sistema (LaunchAgent)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home := os.Getenv("HOME")
			plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mediaos.content.plist")

			if _, err := os.Stat(plistPath); os.IsNotExist(err) {
				fmt.Println("ℹ️  No hay servicio instalado.")
				return nil
			}

			if err := exec.Command("launchctl", "unload", plistPath).Run(); err != nil {
				fmt.Printf("⚠️  launchctl unload: %v\n", err)
			}
			if err := os.Remove(plistPath); err != nil {
				return fmt.Errorf("eliminando plist: %w", err)
			}
			fmt.Println("✅ Servicio eliminado. El programa ya no correrá en background.")
			return nil
		},
	}
}
