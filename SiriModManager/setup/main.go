//go:build windows

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
)

//go:embed SiriModManager.exe app.ico
var payload embed.FS

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if e := win.InitCOM(); e != nil {
		win.Message(0, "Siri ModManager Setup", e.Error(), 0x10)
		return
	}
	defer win.CloseCOM()
	mutex, ok := win.SingleInstance()
	if !ok {
		win.Message(0, "Siri ModManager Setup", "Bitte den laufenden Siri ModManager vor der Installation schließen.", 0x40)
		return
	}
	defer func() {
		if mutex != 0 {
			win.CloseHandle(mutex)
		}
	}()
	if win.Message(0, "Siri ModManager Setup", "Siri ModManager "+core.Version+" installieren?\n\nInstallation für deinen Windows-Benutzer, mit Desktop-Icon und Startmenü-Eintrag. Administratorrechte sind nicht erforderlich.\n\nDeine Mods und Einstellungen bleiben erhalten.", 0x24) != 6 {
		return
	}
	if e := install(); e != nil {
		win.Message(0, "Siri ModManager Setup", e.Error(), 0x10)
		return
	}
	win.CloseHandle(mutex)
	mutex = 0
	if win.Message(0, "Siri ModManager Setup", "Installation abgeschlossen.\n\nDesktop-Icon und Startmenü-Eintrag wurden erstellt.\n\nModManager jetzt starten?", 0x24) == 6 {
		_ = win.Open(0, filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Siri ModManager", "SiriModManager.exe"))
	}
}
func install() error {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return fmt.Errorf("Windows-Anwendungsordner wurde nicht gefunden")
	}
	dir := filepath.Join(local, "Programs", "Siri ModManager")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	data, e := payload.ReadFile("SiriModManager.exe")
	if e != nil {
		return e
	}
	icon, e := payload.ReadFile("app.ico")
	if e != nil {
		return e
	}
	target := filepath.Join(dir, "SiriModManager.exe")
	tmp := target + ".new"
	if e = os.WriteFile(tmp, data, 0755); e != nil {
		return e
	}
	defer os.Remove(tmp)
	backup := target + ".old"
	exists := false
	if _, e = os.Stat(target); e == nil {
		exists = true
		if _, e = os.Stat(backup); e == nil {
			if e = os.Remove(backup); e != nil {
				return e
			}
		}
		if e = os.Rename(target, backup); e != nil {
			return fmt.Errorf("Alte Version konnte nicht ersetzt werden. Bitte den Manager schließen: %w", e)
		}
	}
	if e = os.Rename(tmp, target); e != nil {
		if exists {
			_ = os.Rename(backup, target)
		}
		return e
	}
	_ = os.Remove(backup)
	iconPath := filepath.Join(dir, "app.ico")
	if e = os.WriteFile(iconPath, icon, 0644); e != nil {
		return e
	}
	if e = win.DesktopShortcut(target, iconPath); e != nil {
		return fmt.Errorf("Programm installiert, Desktop-Verknüpfung fehlgeschlagen: %w", e)
	}
	programs, e := win.KnownFolder(2)
	if e != nil {
		return e
	}
	if e = win.Shortcut(target, filepath.Join(programs, "Siri ModManager.lnk"), iconPath); e != nil {
		return e
	}
	return nil
}
