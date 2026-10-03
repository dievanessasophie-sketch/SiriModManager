//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
	"time"
)

func accountClient(timeout time.Duration) *http.Client { return app.auth.Client(timeout) }
func clearPrivateView() {
	app.mods = nil
	app.online = false
	app.pager = 0
	app.authenticated = false
	app.username = ""
	if detail != nil {
		pDestroyWindow.Call(detail.hwnd)
	}
	fillCategories()
	layout()
	invalidate()
}
func handleAuthError(e error) {
	if errors.Is(e, core.ErrLoginRequired) || errors.Is(e, core.ErrVerificationRequired) || errors.Is(e, core.ErrMFARequired) {
		clearPrivateView()
	}
}
func login() {
	if !app.ready || app.auth == nil || app.authPending || app.busy || app.fetching {
		return
	}
	app.authPending = true
	app.status = "Bestätige die Verbindung in deinem Browser …"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	app.authCancel = cancel
	auth := app.auth
	invalidate()
	worker(func() {
		device, _ := os.Hostname()
		if device == "" {
			device = "Windows-PC"
		}
		e := auth.Login(ctx, device, func(raw string) error { return win.Open(0, raw) })
		cancel()
		name, signed, _ := auth.Snapshot()
		post(func() {
			app.authPending = false
			app.authCancel = nil
			app.authenticated = signed
			app.username = name
			layout()
			if e != nil {
				app.status = e.Error()
				footerError = e.Error()
				invalidate()
				return
			}
			app.status = "Verbunden als " + name
			refresh()
			invalidate()
		})
	})
}
func logout() {
	if app.auth == nil || app.authPending {
		return
	}
	if app.busy || app.fetching {
		app.status = "Bitte den laufenden Vorgang abwarten oder abbrechen."
		invalidate()
		return
	}
	app.authPending = true
	clearPrivateView()
	app.status = "Dieser PC wird abgemeldet …"
	auth := app.auth
	worker(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e := auth.Logout(ctx)
		post(func() {
			app.authPending = false
			app.status = "Abgemeldet. Installierte Mods bleiben auf deinem PC."
			if e != nil {
				app.status = e.Error()
				win.Message(hwndMain, "Abmeldung", e.Error(), 0x40)
			}
			invalidate()
		})
	})
}
func openAccountLink(kind string) {
	if app.auth == nil || app.authPending {
		return
	}
	auth := app.auth
	worker(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_, _, d := auth.Snapshot()
		var e error
		if d.Devices == "" {
			d, e = auth.Discover(ctx)
		}
		if e != nil {
			post(func() { fail(e) })
			return
		}
		raw := d.Devices
		switch kind {
		case "verify":
			raw = d.Verify
		case "register":
			raw = d.Register
		case "modbase":
			raw = d.Modbase
		}
		if e = win.Open(0, raw); e != nil {
			post(func() { fail(fmt.Errorf("Browser konnte nicht geöffnet werden.")) })
		}
	})
}
