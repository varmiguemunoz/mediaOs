package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

func Desktop(title, message string) {
	if runtime.GOOS != "darwin" {
		fmt.Printf("[NOTIFY] %s: %s\n", title, message)
		return
	}
	script := fmt.Sprintf(
		`display notification %q with title %q sound name "Glass"`,
		message, title,
	)
	_ = exec.Command("osascript", "-e", script).Run()
}
