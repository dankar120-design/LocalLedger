package api

import (
	"log"
	"os/exec"
	"runtime"
)

// OpenBrowserAppMode försöker starta angiven URL i en "App-liknande" miljö (utan flikar och adressfält)
// Om varken Edge eller Chrome hittas i App-mode faller den tillbaka på systemets standardwebbläsare.
func OpenBrowserAppMode(url string) {
	if runtime.GOOS == "windows" {
		// Först testar vi Edge App Mode
		cmdEdge := exec.Command("cmd", "/c", "start", "msedge", "--app="+url)
		if err := cmdEdge.Run(); err == nil {
			return
		}

		// Om Edge misslyckas, testa Chrome App Mode
		cmdChrome := exec.Command("cmd", "/c", "start", "chrome", "--app="+url)
		if err := cmdChrome.Run(); err == nil {
			return
		}

		// Standard fallback
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	} else if runtime.GOOS == "darwin" {
		exec.Command("open", url).Start()
	} else if runtime.GOOS == "linux" {
		exec.Command("xdg-open", url).Start()
	} else {
		log.Printf("Vänligen öppna din webbläsare på: %s", url)
	}
}
