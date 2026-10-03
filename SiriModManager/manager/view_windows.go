//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
	"strings"
	"unsafe"
)

type palette struct{ bg, side, panel, line, text, muted, accent, accentText, green, warn uintptr }

func colors() palette {
	if app.cfg.Theme == "light" {
		return palette{rgb(242, 246, 249), rgb(229, 237, 242), rgb(255, 255, 255), rgb(206, 218, 227), rgb(25, 39, 51), rgb(79, 99, 114), rgb(8, 111, 124), rgb(255, 255, 255), rgb(18, 121, 78), rgb(149, 93, 0)}
	}
	return palette{rgb(9, 15, 24), rgb(13, 21, 31), rgb(20, 30, 43), rgb(36, 51, 68), rgb(241, 245, 249), rgb(151, 169, 186), rgb(69, 213, 197), rgb(7, 28, 33), rgb(125, 224, 177), rgb(246, 191, 99)}
}
func text(h uintptr, s string, r RECT, color, font uintptr) {
	drawText(h, s, r, color, font, DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
}
func btn(h uintptr, r RECT, s string, primary bool, fn func()) {
	c := colors()
	bg, fg := c.panel, c.text
	if primary {
		bg, fg = c.accent, c.accentText
	}
	fillRound(h, r, bg, 12)
	if !primary {
		outlineRound(h, r, c.line, 1, 12)
	}
	drawText(h, s, r, fg, fontSemibold, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	if fn != nil {
		hits = append(hits, hit{r, fn})
	}
}
func pill(h uintptr, r RECT, s string, fg uintptr) {
	c := colors()
	fillRound(h, r, c.bg, 10)
	drawText(h, s, r, fg, fontRegular, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}
func paint(hwnd uintptr) {
	var ps PAINTSTRUCT
	dc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var r RECT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if r.Right < 1 || r.Bottom < 1 {
		return
	}
	mem, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(dc)
	bm, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(dc, uintptr(r.Right), uintptr(r.Bottom))
	old, _, _ := pSelectObject.Call(mem, bm)
	defer func() {
		gdi32.NewProc("BitBlt").Call(dc, 0, 0, uintptr(r.Right), uintptr(r.Bottom), mem, 0, 0, SRCCOPY)
		pSelectObject.Call(mem, old)
		pDeleteObject.Call(bm)
		gdi32.NewProc("DeleteDC").Call(mem)
	}()
	c := colors()
	fillRound(mem, r, c.bg, 0)
	hits = nil
	sidebar(mem, r)
	if app.page == 4 || (app.page != 1 && app.page != 3 && !app.authenticated) {
		accountPage(mem, r)
	} else if app.page == 3 {
		settings(mem, r)
	} else {
		catalog(mem, r)
	}
	statusBar(mem, r)
}
func sidebar(h uintptr, r RECT) {
	c := colors()
	fillRound(h, RECT{0, 0, 220, r.Bottom}, c.side, 0)
	fillRound(h, RECT{24, 28, 64, 68}, c.accent, 12)
	drawText(h, "S", RECT{24, 27, 64, 67}, c.accentText, fontBold, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(h, "SIRI", RECT{76, 27, 205, 57}, c.text, fontLogo)
	text(h, "MODMANAGER", RECT{76, 55, 211, 79}, c.muted, fontRegular)
	line(h, 24, 103, 196, 103, c.line, 1)
	labels := []string{"Modbase", "Installiert", "Updates", "Einstellungen", "Forumkonto"}
	for i, s := range labels {
		y := int32(125 + i*55)
		rr := RECT{16, y, 204, y + 43}
		if app.page == i {
			fillRound(h, rr, c.panel, 12)
			fillRound(h, RECT{16, y + 10, 19, y + 33}, c.accent, 2)
		}
		text(h, fmt.Sprintf("%02d", i+1), RECT{30, y + 10, 58, y + 33}, c.accent, fontRegular)
		text(h, s, RECT{67, y + 10, 198, y + 35}, c.text, fontSemibold)
		idx := i
		hits = append(hits, hit{rr, func() { setPage(idx) }})
	}
	if r.Bottom > 720 {
		text(h, "TRANSPORT FEVER", RECT{28, 425, 204, 450}, c.muted, fontRegular)
		text(h, "2  +  3", RECT{28, 458, 195, 489}, c.text, fontBold)
	}
	btn(h, RECT{24, r.Bottom - 165, 196, r.Bottom - 127}, "Modbase öffnen ↗", false, func() {
		if e := win.Open(hwndMain, core.ModbaseURL); e != nil {
			fail(e)
		}
	})
	status := "ANMELDUNG NÖTIG"
	if app.authenticated {
		status = "VERBUNDEN / OFFLINE"
	}
	col := c.warn
	if app.online {
		status = "MODBASE ONLINE"
		col = c.green
	}
	if app.fetching {
		status = "WIRD ABGEFRAGT …"
		col = c.accent
	}
	text(h, "●  "+status, RECT{26, r.Bottom - 103, 211, r.Bottom - 77}, col, fontRegular)
	text(h, "siri-mods.de · v"+core.Version, RECT{26, r.Bottom - 66, 213, r.Bottom - 40}, c.muted, fontRegular)
}

type row struct {
	m core.Mod
	i *core.Installed
}

func rows() []row {
	out := []row{}
	ins := installedMap()
	if app.page == 1 {
		for _, i := range app.installed {
			m := core.Mod{ID: i.ID, Name: i.Name, Game: i.Game, Version: i.Version, Category: "Lokale Mods", Folder: i.Folder}
			for _, v := range app.mods {
				if v.Key() == i.Key() {
					m = v
					break
				}
			}
			ii := i
			out = append(out, row{m, &ii})
		}
	} else {
		for _, m := range app.mods {
			i, ok := ins[m.Key()]
			if app.page == 2 && (!ok || !core.NeedsUpdate(m, i)) {
				continue
			}
			var ip *core.Installed
			if ok {
				ii := i
				ip = &ii
			}
			out = append(out, row{m, ip})
		}
	}
	result := []row{}
	q := strings.ToLower(strings.TrimSpace(app.query))
	for _, v := range out {
		m := v.m
		if app.game != "" && m.Game != app.game {
			continue
		}
		if app.category != "" && m.Category != app.category {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.Name+" "+m.Description+" "+m.Author+" "+m.Category+" "+m.Game), q) {
			continue
		}
		result = append(result, v)
	}
	return result
}
func catalog(h uintptr, r RECT) {
	c := colors()
	titles := []string{"Deine Mods. Deine Welt.", "Installierte Mods", "Deine Updates"}
	subs := []string{"Veröffentlichungen aus der Siri Modbase – direkt auf deinen PC.", "Verwaltete und erkannte Mods in deinen ausgewählten Ordnern.", "Neue Versionen werden beim Start und regelmäßig erkannt."}
	text(h, titles[app.page], RECT{252, 30, r.Right - 236, 70}, c.text, fontHero)
	text(h, subs[app.page], RECT{254, 79, r.Right - 32, 103}, c.muted, fontRegular)
	label := "Aktualisieren"
	if app.fetching {
		label = "Wird geprüft …"
	}
	btn(h, RECT{r.Right - 203, 34, r.Right - 32, 77}, label, false, refresh)
	stats := []struct {
		n string
		v int
	}{{"Veröffentlichungen", len(app.mods)}, {"Installierte Mods", len(app.installed)}, {"Updates verfügbar", len(updates())}}
	cw := (r.Right - 284 - 24) / 3
	for j, s := range stats {
		x := int32(252) + int32(j)*(cw+12)
		rr := RECT{x, 121, x + cw, 193}
		fillRound(h, rr, c.panel, 14)
		text(h, s.n, RECT{x + 16, 135, x + cw - 60, 159}, c.muted, fontRegular)
		drawText(h, fmt.Sprint(s.v), RECT{x + cw - 63, 135, x + cw - 17, 173}, c.accent, fontBold, DT_RIGHT|DT_SINGLELINE)
	}
	list := rows()
	columns := 2
	width := r.Right - 284
	cardW := (width - 16) / 2
	rowCount := int((r.Bottom - 365) / 174)
	if rowCount < 1 {
		rowCount = 1
	}
	per := columns * rowCount
	pages := (len(list) + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	if app.pager >= pages {
		app.pager = pages - 1
	}
	text(h, fmt.Sprintf("%d Einträge", len(list)), RECT{254, 272, 560, 298}, c.muted, fontRegular)
	if app.page == 2 && len(updates()) > 0 {
		btn(h, RECT{r.Right - 267, 265, r.Right - 32, 300}, "Alle Updates installieren", true, func() { installMany(updates()) })
	}
	if len(list) == 0 {
		title, body := "Keine Treffer", "Passe Suche, Spiel oder Kategorie an."
		if len(app.mods) == 0 && app.page == 0 {
			title = "Deine Modbase wartet auf Verbindung"
			body = "Klicke auf Aktualisieren. Bei Verbindungsproblemen findest du Details unter Einstellungen → Protokoll."
		}
		if app.page == 1 {
			title = "Noch keine Mods erkannt"
			body = "Wähle unter Einstellungen die Mods-Ordner von TF2 und TF3. Vorhandene Mods werden dann eingelesen."
		}
		if app.page == 2 {
			title = "Keine bekannten Updates"
			body = "Nur erkannte, verlässliche Versionsstände lassen sich vergleichen. Ohne Verbindung kann der Stand veraltet sein."
		}
		fillRound(h, RECT{252, 317, r.Right - 32, 490}, c.panel, 18)
		text(h, title, RECT{276, 343, r.Right - 56, 384}, c.text, fontBold)
		drawText(h, body, RECT{276, 395, r.Right - 60, 463}, c.muted, fontRegular, DT_LEFT|DT_WORDBREAK)
		return
	}
	start := app.pager * per
	end := start + per
	if end > len(list) {
		end = len(list)
	}
	for n, v := range list[start:end] {
		x := int32(252) + int32(n%2)*(cardW+16)
		y := int32(312) + int32(n/2)*174
		modCard(h, RECT{x, y, x + cardW, y + 158}, v)
	}
	y := r.Bottom - 90
	text(h, fmt.Sprintf("Seite %d / %d", app.pager+1, pages), RECT{254, y + 10, 540, y + 38}, c.muted, fontRegular)
	if app.pager > 0 {
		btn(h, RECT{r.Right - 282, y, r.Right - 164, y + 38}, "‹ Zurück", false, func() { app.pager--; invalidate() })
	}
	if app.pager+1 < pages {
		btn(h, RECT{r.Right - 150, y, r.Right - 32, y + 38}, "Weiter ›", false, func() { app.pager++; invalidate() })
	}
}
func modCard(h uintptr, r RECT, v row) {
	c := colors()
	m, i := v.m, v.i
	fillRound(h, r, c.panel, 16)
	outlineRound(h, r, c.line, 1, 16)
	pill(h, RECT{r.Left + 16, r.Top + 15, r.Left + 63, r.Top + 45}, m.Game, c.accent)
	text(h, m.Name, RECT{r.Left + 75, r.Top + 17, r.Right - 16, r.Top + 43}, c.text, fontSemibold)
	hits = append(hits, hit{RECT{r.Left + 75, r.Top + 10, r.Right - 14, r.Top + 47}, func() { showDetails(m, i) }})
	tag := "v" + m.Version + " · " + m.Category
	if m.Version == "" {
		tag = "Noch keine Version · " + m.Category
	}
	if m.Upcoming {
		tag = "Angekündigt · " + m.Category
	}
	if i != nil {
		tag = "Installiert: " + i.Version
		if core.NeedsUpdate(m, *i) {
			tag += "  →  " + m.Version
		} else if !core.VersionKnown(i.Version) {
			tag += " · Version prüfen"
		}
	}
	text(h, tag, RECT{r.Left + 16, r.Top + 55, r.Right - 16, r.Top + 79}, c.muted, fontRegular)
	desc := m.Description
	if desc == "" {
		desc = "Details durch Klick auf den Modnamen."
	}
	text(h, desc, RECT{r.Left + 16, r.Top + 82, r.Right - 16, r.Top + 105}, c.muted, fontRegular)
	primary, label := true, "Installieren"
	action := func() { installMany([]core.Mod{m}) }
	if !m.CanInstall() {
		primary = false
		label = "Modbase öffnen"
		action = func() {
			u := m.URL
			if u == "" {
				u = core.ModbaseURL
			}
			if e := win.Open(hwndMain, u); e != nil {
				fail(e)
			}
		}
	}
	if i != nil {
		if core.NeedsUpdate(m, *i) {
			label = "Aktualisieren"
		} else if !core.VersionKnown(i.Version) && m.CanInstall() {
			label = "Neu installieren"
		} else {
			label = "Ordner öffnen"
			primary = false
			action = func() {
				if len(i.AllFolders()) > 0 {
					if e := win.Open(hwndMain, filepath.Join(i.Root, i.AllFolders()[0])); e != nil {
						fail(e)
					}
				}
			}
		}
	}
	btn(h, RECT{r.Left + 16, r.Top + 116, r.Left + 169, r.Top + 146}, label, primary, action)
	if i != nil {
		ii := *i
		btn(h, RECT{r.Right - 153, r.Top + 116, r.Right - 16, r.Top + 146}, "Deinstallieren", false, func() { uninstall(ii) })
	} else {
		btn(h, RECT{r.Right - 109, r.Top + 116, r.Right - 16, r.Top + 146}, "Details", false, func() { showDetails(m, i) })
	}
}
func settings(h uintptr, r RECT) {
	c := colors()
	left, right := int32(252), r.Right-32
	text(h, "Einstellungen", RECT{left, 30, right, 75}, c.text, fontHero)
	text(h, "Deine Spiele, dein Erscheinungsbild und deine Verbindung.", RECT{left + 2, 82, right, 108}, c.muted, fontRegular)
	btn(h, RECT{right - 184, 34, right, 73}, "Forumkonto", false, func() { setPage(4) })
	for n, g := range []string{"TF2", "TF3"} {
		y := int32(117 + n*120)
		fillRound(h, RECT{left, y, right, y + 110}, c.panel, 16)
		name := "Transport Fever 2"
		hint := "Steam: userdata / Steam-ID / 1066780 / local / mods"
		if g == "TF3" {
			name = "Transport Fever 3"
			hint = "Eigenen lokalen Mods-Ordner auswählen; TF3 getrennt verwalten."
		}
		text(h, name, RECT{left + 18, y + 14, right - 230, y + 42}, c.text, fontBold)
		text(h, hint, RECT{left + 18, y + 49, right - 18, y + 72}, c.muted, fontRegular)
		p := app.cfg.Root(g)
		if p == "" {
			p = "Noch kein Mods-Ordner ausgewählt"
		}
		text(h, p, RECT{left + 18, y + 78, right - 208, y + 105}, c.accent, fontRegular)
		game := g
		btn(h, RECT{right - 190, y + 75, right - 18, y + 105}, "Ordner auswählen …", false, func() { choose(game) })
		if app.cfg.Root(g) != "" {
			path := app.cfg.Root(g)
			btn(h, RECT{right - 136, y + 13, right - 18, y + 43}, "Öffnen ↗", false, func() {
				if e := win.Open(hwndMain, path); e != nil {
					fail(e)
				}
			})
		}
	}
	y := int32(363)
	fillRound(h, RECT{left, y, right, y + 70}, c.panel, 14)
	text(h, "Erscheinungsbild", RECT{left + 18, y + 15, left + 265, y + 42}, c.text, fontSemibold)
	text(h, "Dunkel ist der Standard", RECT{left + 18, y + 42, left + 340, y + 64}, c.muted, fontRegular)
	btn(h, RECT{right - 246, y + 17, right - 140, y + 53}, "Dunkel", app.cfg.Theme != "light", func() { app.cfg.Theme = "dark"; saveConfig(); updateTheme() })
	btn(h, RECT{right - 125, y + 17, right - 18, y + 53}, "Hell", app.cfg.Theme == "light", func() { app.cfg.Theme = "light"; saveConfig(); updateTheme() })
	y = 448
	fillRound(h, RECT{left, y, right, y + 69}, c.panel, 14)
	text(h, "Automatische Update-Prüfung", RECT{left + 18, y + 12, right - 165, y + 37}, c.text, fontSemibold)
	text(h, "Beim Start und alle 15 Minuten, solange der Manager geöffnet ist.", RECT{left + 18, y + 39, right - 165, y + 63}, c.muted, fontRegular)
	s := "Aus"
	if app.cfg.AutoCheck {
		s = "Ein"
	}
	btn(h, RECT{right - 124, y + 17, right - 18, y + 52}, s, app.cfg.AutoCheck, func() { app.cfg.AutoCheck = !app.cfg.AutoCheck; saveConfig(); invalidate() })
	y = 532
	btn(h, RECT{left, y, left + 200, y + 38}, "Desktop-Icon erstellen", false, createShortcut)
	btn(h, RECT{left + 214, y, left + 388, y + 38}, "Mods neu einlesen", false, rescan)
	btn(h, RECT{left + 402, y, left + 539, y + 38}, "Protokoll öffnen", false, func() {
		p := appPath("manager.log")
		if _, e := os.Stat(p); os.IsNotExist(e) {
			_ = os.WriteFile(p, []byte("Siri ModManager – noch keine Fehler protokolliert.\r\n"), 0644)
		}
		if e := win.Open(hwndMain, p); e != nil {
			fail(e)
		}
	})
	if r.Bottom > 685 {
		text(h, app.cfg.APIURL, RECT{left + 2, 598, right, 621}, c.muted, fontRegular)
	}
}
func statusBar(h uintptr, r RECT) {
	c := colors()
	y := r.Bottom - 41
	fillRound(h, RECT{220, y, r.Right, r.Bottom}, c.side, 0)
	if app.progress >= 0 {
		p := app.progress
		if p > 1 {
			p = 1
		}
		fillRound(h, RECT{220, y, 220 + int32(float64(r.Right-220)*p), y + 3}, c.accent, 0)
	}
	right := r.Right - 22
	if app.cancel != nil {
		right -= 125
		btn(h, RECT{r.Right - 127, y + 6, r.Right - 16, r.Bottom - 6}, "Abbrechen", false, func() {
			if app.cancel != nil {
				app.cancel()
				app.status = "Abbruch wird ausgeführt …"
				invalidate()
			}
		})
	}
	text(h, app.status, RECT{252, y + 12, right, r.Bottom - 7}, c.muted, fontRegular)
}

func accountPage(h uintptr, r RECT) {
	c := colors()
	left, right := int32(252), r.Right-32
	text(h, "Dein Forumkonto.", RECT{left, 34, right, 79}, c.text, fontHero)
	text(h, "Ein Konto für ModBase und ModManager.", RECT{left + 2, 89, right, 118}, c.muted, fontRegular)
	fillRound(h, RECT{left, 145, right, 407}, c.panel, 18)
	heading, body := "Im Forum anmelden", "Verbinde diesen PC mit deinem registrierten und freigeschalteten Forumkonto. Die Anmeldung öffnet deinen Browser."
	if app.authenticated {
		heading = "Verbunden als " + app.username
		body = "Deine Freischaltung und die Rechte deiner Benutzergruppen gelten auch hier. Downloads und Updates werden vom Forum geprüft."
	}
	if app.authPending {
		heading = "Verbindung wird bearbeitet …"
		body = "Bestätige die Gerätefreigabe im Browser. Du kannst die Anmeldung hier abbrechen und später neu starten."
	}
	text(h, heading, RECT{left + 24, 172, right - 24, 214}, c.text, fontBold)
	drawText(h, body, RECT{left + 24, 234, right - 24, 301}, c.muted, fontRegular, DT_LEFT|DT_WORDBREAK)
	y := int32(334)
	if app.authPending {
		if app.authCancel != nil {
			btn(h, RECT{left + 24, y, left + 239, y + 42}, "Anmeldung abbrechen", false, func() { app.authCancel() })
		}
	} else if app.authenticated {
		btn(h, RECT{left + 24, y, left + 248, y + 42}, "Geräte verwalten ↗", true, func() { openAccountLink("devices") })
		btn(h, RECT{left + 264, y, left + 437, y + 42}, "Abmelden", false, logout)
	} else {
		btn(h, RECT{left + 24, y, left + 248, y + 42}, "Im Forum anmelden ↗", true, login)
		btn(h, RECT{left + 264, y, left + 437, y + 42}, "Registrieren ↗", false, func() { openAccountLink("register") })
	}
	text(h, "Deine Freischaltung bleibt maßgeblich", RECT{left + 2, 435, right, 470}, c.text, fontSemibold)
	drawText(h, "Noch kein Zugang? Nach der Registrierung muss dein Forumkonto für die ModBase freigeschaltet werden. Bereits installierte Mods kannst du unter „Installiert“ weiterhin verwalten.", RECT{left + 2, 482, right - 12, 547}, c.muted, fontRegular, DT_LEFT|DT_WORDBREAK)
	btn(h, RECT{left, 550, left + 245, 589}, "Freischaltung ansehen ↗", false, func() { openAccountLink("verify") })
	if !app.authenticated {
		btn(h, RECT{left + 260, 550, left + 490, 589}, "Geräte verwalten ↗", false, func() { openAccountLink("devices") })
	}
}
