//go:build windows

package main

import (
	"context"
	"fmt"
	"net/http"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
	"time"
)

var checkingManagerUpdate bool
var managerUpdate core.ManagerUpdate
var managerUpdateStatus = "Noch nicht geprüft."

func checkManagerUpdate(manual bool) {
	if checkingManagerUpdate {
		return
	}
	checkingManagerUpdate = true
	managerUpdateStatus = "Suche nach einer neuen ModManager-Version …"
	invalidate()
	worker(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		update, err := core.CheckManagerUpdate(ctx, client, core.Version)
		post(func() {
			checkingManagerUpdate = false
			if err != nil {
				managerUpdateStatus = err.Error()
				logError(err)
				invalidate()
				if manual {
					win.Message(hwndMain, "ModManager-Updates", managerUpdateStatus, 0x30)
				}
				return
			}
			managerUpdate = update
			managerUpdateStatus = "Keine neuere veröffentlichte Version gefunden."
			if update.Available {
				managerUpdateStatus = "Version " + update.Version + " ist verfügbar."
			}
			invalidate()
			if update.Available {
				message := fmt.Sprintf("Siri ModManager %s ist verfügbar. Installiert: %s.\n\nDownloadseite öffnen? Lade dort das Setup herunter und schließe den Manager vor der Installation.", update.Version, core.Version)
				if win.Message(hwndMain, "ModManager-Update verfügbar", message, 0x24) == 6 {
					openManagerRelease()
				}
			} else if manual {
				win.Message(hwndMain, "ModManager-Updates", managerUpdateStatus, 0x40)
			}
		})
	})
}
func openManagerRelease() {
	url := core.ManagerReleasesURL
	if managerUpdate.Available {
		url = managerUpdate.URL
	}
	if err := win.Open(hwndMain, url); err != nil {
		fail(err)
	}
}
func managerUpdatePage(h uintptr, r RECT) {
	c := colors()
	left, right := int32(252), r.Right-32
	text(h, "ModManager-Updates", RECT{left, 30, right - 200, 75}, c.text, fontHero)
	btn(h, RECT{right - 184, 34, right, 73}, "Zurück", false, func() { setPage(3) })
	text(h, "Installierte Version: "+core.Version, RECT{left, 90, right, 120}, c.muted, fontRegular)
	fillRound(h, RECT{left, 145, right, 310}, c.panel, 16)
	text(h, "Programm beim Start auf Updates prüfen", RECT{left + 18, 165, right - 145, 195}, c.text, fontSemibold)
	label := "Aus"
	if app.cfg.ManagerAutoCheck {
		label = "Ein"
	}
	btn(h, RECT{right - 124, 162, right - 18, 199}, label, app.cfg.ManagerAutoCheck, func() { app.cfg.ManagerAutoCheck = !app.cfg.ManagerAutoCheck; saveConfig(); invalidate() })
	drawText(h, "Die Prüfung verwendet öffentliche GitHub Releases. Dein Forumkonto und deine Moddaten werden dabei nicht übertragen. Die Installation startest du selbst über das neue Setup.", RECT{left + 18, 218, right - 18, 293}, c.muted, fontRegular, DT_LEFT|DT_WORDBREAK)
	drawText(h, managerUpdateStatus, RECT{left, 338, right, 395}, c.text, fontRegular, DT_LEFT|DT_WORDBREAK)
	var check func()
	label = "Jetzt prüfen"
	if checkingManagerUpdate {
		label = "Prüfung läuft …"
	} else {
		check = func() { checkManagerUpdate(true) }
	}
	btn(h, RECT{left, 418, left + 190, 460}, label, false, check)
	btn(h, RECT{left + 205, 418, left + 440, 460}, "Downloadseite öffnen ↗", false, openManagerRelease)
}
