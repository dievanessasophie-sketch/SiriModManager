//go:build windows

package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

//go:embed app.ico
var assets embed.FS
var hwndMain, searchEdit, gameCombo, categoryCombo uintptr
var uiQueue = make(chan func(), 512)
var closing atomic.Bool
var app = appState{cfg: core.DefaultConfig(), status: "ModManager wird geladen …", progress: -1}

type appState struct {
	auth                                         *core.Auth
	username                                     string
	authenticated, authPending                   bool
	authCancel                                   context.CancelFunc
	cfg                                          core.Config
	mods                                         []core.Mod
	installed                                    []core.Installed
	page, pager                                  int
	query, game, category                        string
	categories                                   []string
	status                                       string
	busy, fetching, online, ready, rescanPending bool
	lastSync                                     time.Time
	progress                                     float64
	cancel                                       context.CancelFunc
}
type hit struct {
	r  RECT
	fn func()
}

var hits []hit
var iconHandle uintptr
var footerError string

const wmTasks = 0x8002
const wmClose = 0x0010
const wmMinMax = 0x0024
const cbAdd = 0x143
const cbSel = 0x147
const cbSetSel = 0x14E
const cbReset = 0x14B

func post(fn func()) {
	if closing.Load() {
		return
	}
	select {
	case uiQueue <- fn:
		pPostMessageW.Call(hwndMain, wmTasks, 0, 0)
	default: // Progress can wait; all state changes remain ordered.
		select {
		case uiQueue <- fn:
			pPostMessageW.Call(hwndMain, wmTasks, 0, 0)
		case <-time.After(3 * time.Second):
			logError(fmt.Errorf("Oberflächenwarteschlange blockiert"))
		}
	}
}
func invalidate()             { pInvalidateRect.Call(hwndMain, 0, 0) }
func appPath(n string) string { return filepath.Join(win.AppDir(), n) }
func logError(e error) {
	_ = os.MkdirAll(win.AppDir(), 0755)
	f, err := os.OpenFile(appPath("manager.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		fmt.Fprintf(f, "%s %v\n", time.Now().Format(time.RFC3339), e)
		f.Close()
	}
}
func fail(e error) {
	if e == nil {
		return
	}
	logError(e)
	app.status = e.Error()
	footerError = e.Error()
	invalidate()
	win.Message(hwndMain, "Siri ModManager", e.Error(), 0x10)
}
func saveConfig() {
	if e := core.WriteJSON(appPath("config.json"), app.cfg); e != nil {
		fail(fmt.Errorf("Einstellungen konnten nicht gespeichert werden: %w", e))
	}
}
func installedMap() map[string]core.Installed {
	m := map[string]core.Installed{}
	for _, i := range app.installed {
		m[i.Key()] = i
	}
	return m
}
func updates() []core.Mod {
	ins := installedMap()
	out := []core.Mod{}
	for _, m := range app.mods {
		if i, ok := ins[m.Key()]; ok && core.NeedsUpdate(m, i) {
			out = append(out, m)
		}
	}
	return out
}
func worker(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e := fmt.Errorf("Interner Fehler: %v\n%s", r, debug.Stack())
				logError(e)
				post(func() {
					app.busy = false
					app.fetching = false
					app.cancel = nil
					app.status = "Interner Fehler – siehe manager.log"
					invalidate()
				})
			}
		}()
		fn()
	}()
}
func beginJob(status string) bool {
	if app.busy {
		app.status = "Bitte den laufenden Vorgang abwarten."
		invalidate()
		return false
	}
	app.busy = true
	app.progress = -1
	app.status = status
	footerError = ""
	invalidate()
	return true
}
func endJob() {
	app.busy = false
	app.progress = -1
	app.cancel = nil
	invalidate()
	if app.rescanPending {
		app.rescanPending = false
		rescan()
	}
}
func progress(label string, n, total int64) {
	post(func() {
		app.status = label
		if n > 0 {
			app.status += fmt.Sprintf(" · %.1f MiB", float64(n)/(1<<20))
			if total > 0 {
				app.status += fmt.Sprintf(" / %.1f MiB", float64(total)/(1<<20))
				app.progress = float64(n) / float64(total)
			}
		}
		invalidate()
	})
}
func roots() []string {
	out := []string{`C:\Program Files (x86)\Steam`, `C:\Program Files\Steam`}
	for _, v := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if p := os.Getenv(v); p != "" {
			out = append(out, filepath.Join(p, "Steam"))
		}
	}
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, win.Ptr(`Software\Valve\Steam`), 0, syscall.KEY_READ, &key) == nil {
		defer syscall.RegCloseKey(key)
		var typ, n uint32
		n = 32768
		buf := make([]uint16, n/2)
		if syscall.RegQueryValueEx(key, win.Ptr("SteamPath"), nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &n) == nil {
			out = append(out, syscall.UTF16ToString(buf))
		}
	}
	return out
}
func load() {
	worker(func() {
		cfg := core.DefaultConfig()
		var notes []string
		if e := core.ReadJSON(appPath("config.json"), &cfg); e != nil && !os.IsNotExist(e) {
			notes = append(notes, "Alte Einstellungen konnten nicht gelesen werden.")
		}
		if cfg.APIURL == "" {
			cfg.APIURL = core.APIURL
		}
		if cfg.Theme != "light" {
			cfg.Theme = "dark"
		}
		// The old public catalog is intentionally discarded; private metadata stays in memory.
		_ = os.Remove(appPath("mods_cache.json"))
		auth, authErr := core.NewAuth(cfg.APIURL, win.SecretStore{Path: appPath("account.dpapi")})
		var username string
		var signed bool
		if authErr != nil {
			notes = append(notes, authErr.Error())
		} else {
			username, signed, _ = auth.Snapshot()
		}
		var old []core.Installed
		_ = core.ReadJSON(appPath("installed.json"), &old)
		q := strings.ToLower(strings.ReplaceAll(cfg.TF2Path, "\\", "/"))
		if cfg.TF2Path == "" || strings.Contains(strings.ToUpper(cfg.TF2Path), "DEINEID") || strings.Contains(q, "/steamapps/common/") {
			matches := core.DetectTF2(roots())
			cfg.TF2Path = ""
			if len(matches) == 1 {
				cfg.TF2Path = matches[0]
			}
		}
		if cfg.TF2Path != "" && strings.EqualFold(cfg.TF2Path, cfg.TF3Path) {
			cfg.TF3Path = ""
			notes = append(notes, "TF3-Modordner bitte getrennt auswählen.")
		}
		for j := range old {
			if old[j].Root == "" {
				old[j].Root = cfg.Root(old[j].Game)
			}
		}
		post(func() {
			app.cfg = cfg
			app.auth = auth
			app.username = username
			app.authenticated = signed
			app.installed = old
			app.ready = true
			app.status = "Bitte im Forum anmelden. Installierte Mods sind lokal verfügbar."
			if len(notes) > 0 {
				app.status = strings.Join(notes, " ")
			}
			updateTheme()
			fillCategories()
			rescan()
			if signed {
				refresh()
			}
		})
	})
}
func refresh() {
	if !app.ready || app.fetching || app.authPending {
		return
	}
	if app.auth == nil || !app.authenticated {
		app.status = core.ErrLoginRequired.Error()
		invalidate()
		return
	}
	app.fetching = true
	if !app.busy {
		app.status = "Modbase und neue Versionen werden geprüft …"
	}
	raw := app.cfg.APIURL
	client := accountClient(25 * time.Second)
	invalidate()
	worker(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		mods, e := core.FetchCatalog(ctx, client, raw)
		post(func() {
			app.fetching = false
			if e != nil {
				app.online = false
				handleAuthError(e)
				footerError = e.Error()
				logError(e)
				if !app.busy {
					app.status = e.Error()
				}
			} else {
				app.online = true
				app.mods = mods
				app.lastSync = time.Now()
				fillCategories()
				app.pager = 0
				if !app.busy {
					app.status = fmt.Sprintf("%d Modbase-Einträge geladen · %d Updates", len(mods), len(updates()))
				}
				if app.busy {
					app.rescanPending = true
				} else {
					rescan()
				}
			}
			invalidate()
		})
	})
}
func rescan() {
	if !app.ready {
		return
	}
	if app.busy {
		app.rescanPending = true
		return
	}
	beginJob("Installierte Mods werden geprüft …")
	cfg, mods, old := app.cfg, append([]core.Mod(nil), app.mods...), append([]core.Installed(nil), app.installed...)
	worker(func() {
		var all []core.Installed
		var warnings []string
		for _, g := range []string{"TF2", "TF3"} {
			list, e := core.Scan(cfg.Root(g), g, mods, old)
			if e != nil {
				warnings = append(warnings, g+": "+e.Error())
			}
			all = append(all, list...)
		}
		e := core.WriteJSON(appPath("installed.json"), all)
		if e != nil {
			warnings = append(warnings, e.Error())
		}
		post(func() {
			app.installed = all
			app.status = fmt.Sprintf("%d installierte Mods erkannt · %d Updates", len(all), len(updates()))
			if len(warnings) > 0 {
				app.status = strings.Join(warnings, " | ")
				logError(fmt.Errorf("%s", app.status))
			}
			endJob()
		})
	})
}
func installMany(mods []core.Mod) {
	if !app.authenticated || app.auth == nil {
		app.status = core.ErrLoginRequired.Error()
		invalidate()
		return
	}
	if len(mods) == 0 {
		return
	}
	if app.busy {
		return
	}
	cfg := app.cfg
	ins := installedMap()
	for _, m := range mods {
		other := cfg.TF3Path
		if m.Game == "TF3" {
			other = cfg.TF2Path
		}
		if e := core.ValidateRoot(cfg.Root(m.Game), other); e != nil {
			fail(fmt.Errorf("%s: %w", m.Game, e))
			setPage(3)
			return
		}
	}
	if len(mods) == 1 {
		m := mods[0]
		if i, ok := ins[m.Key()]; ok {
			prompt := m.Name + " auf Version " + m.Version + " aktualisieren?\n\nBisherige Modordner werden vollständig ersetzt. Eigene Änderungen im Modordner gehen verloren. Bitte das Spiel vorher schließen.\n\n" + i.Root + "\n" + strings.Join(i.AllFolders(), "\n")
			if win.Message(hwndMain, "Mod ersetzen", prompt, 0x24) != 6 {
				return
			}
		}
	} else {
		if win.Message(hwndMain, "Alle Updates installieren", fmt.Sprintf("%d Mods aktualisieren?\n\nDie bisherigen Modordner werden vollständig ersetzt. Bitte vorher das Spiel schließen.", len(mods)), 0x24) != 6 {
			return
		}
	}
	if !beginJob("Download wird gestartet …") {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := accountClient(30 * time.Minute)
	app.cancel = cancel
	worker(func() {
		var changed []core.Installed
		var err error
		for _, m := range mods {
			var old *core.Installed
			if i, ok := ins[m.Key()]; ok {
				copy := i
				old = &copy
			}
			in, e := core.Install(ctx, client, m, cfg.Root(m.Game), old, func(s string, n, total int64) { progress(m.Name+" · "+s, n, total) })
			if e != nil {
				err = e
				break
			}
			changed = append(changed, in)
		}
		cancel()
		post(func() {
			for _, i := range changed {
				found := false
				for n, v := range app.installed {
					if v.Key() == i.Key() {
						app.installed[n] = i
						found = true
						break
					}
				}
				if !found {
					app.installed = append(app.installed, i)
				}
			}
			if e := core.WriteJSON(appPath("installed.json"), app.installed); e != nil {
				logError(e)
			}
			app.status = fmt.Sprintf("%d Mod(s) erfolgreich installiert.", len(changed))
			endJob()
			if err != nil {
				handleAuthError(err)
				if err == context.Canceled {
					app.status = "Download abgebrochen. Vorherige Mods bleiben erhalten."
					invalidate()
				} else {
					fail(err)
				}
			}
		})
	})
}
func uninstall(i core.Installed) {
	if app.busy {
		return
	}
	if win.Message(hwndMain, "Mod deinstallieren", i.Name+" wirklich deinstallieren?\n\nDiese Modordner werden entfernt:\n"+i.Root+"\n"+strings.Join(i.AllFolders(), "\n")+"\n\nSpielstände bleiben erhalten. Bitte das Spiel vorher schließen.", 0x24) != 6 {
		return
	}
	if !beginJob("Mod wird deinstalliert …") {
		return
	}
	worker(func() {
		e := core.Uninstall(i)
		post(func() {
			if e == nil {
				next := []core.Installed{}
				for _, v := range app.installed {
					if v.Key() != i.Key() {
						next = append(next, v)
					}
				}
				app.installed = next
				if se := core.WriteJSON(appPath("installed.json"), next); se != nil {
					logError(se)
				}
				app.status = i.Name + " wurde deinstalliert."
			}
			endJob()
			if e != nil {
				fail(e)
			}
		})
	})
}
func choose(game string) {
	if app.busy {
		return
	}
	current := app.cfg.Root(game)
	p, e := win.PickFolder(hwndMain, game+" · persönlichen Mods-Ordner auswählen", current)
	if e != nil {
		fail(e)
		return
	}
	if p == "" {
		return
	}
	other := app.cfg.TF3Path
	if game == "TF3" {
		other = app.cfg.TF2Path
	}
	if e = core.ValidateRoot(p, other); e != nil {
		fail(e)
		return
	}
	if game == "TF2" {
		app.cfg.TF2Path = p
	} else {
		app.cfg.TF3Path = p
	}
	saveConfig()
	rescan()
	invalidate()
}
func createShortcut() {
	exe, e := os.Executable()
	if e == nil {
		e = win.DesktopShortcut(exe, appPath("app.ico"))
	}
	if e != nil {
		fail(e)
	} else {
		app.status = "Desktop-Icon wurde erstellt."
		invalidate()
	}
}
func getText(h uintptr) string {
	b := make([]uint16, 4096)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func setPage(p int) { app.page = p; app.pager = 0; layout(); invalidate() }
func fillCategories() {
	prev := app.category
	set := map[string]bool{}
	for _, m := range app.mods {
		if m.Category != "" {
			set[m.Category] = true
		}
	}
	app.categories = []string{"Alle Kategorien"}
	for c := range set {
		app.categories = append(app.categories, c)
	}
	sort.Strings(app.categories[1:])
	pSendMessageW.Call(categoryCombo, cbReset, 0, 0)
	sel := 0
	for j, c := range app.categories {
		pSendMessageW.Call(categoryCombo, cbAdd, 0, uintptr(unsafe.Pointer(ptr(c))))
		if c == prev {
			sel = j
		}
	}
	pSendMessageW.Call(categoryCombo, cbSetSel, uintptr(sel), 0)
	if sel == 0 {
		app.category = ""
	}
}
func updateTheme() {
	if editBrush != 0 {
		pDeleteObject.Call(editBrush)
	}
	editBrush = brush(colors().panel)
	var dark int32 = 1
	if app.cfg.Theme == "light" {
		dark = 0
	}
	dwm := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	dwm.Call(hwndMain, 20, uintptr(unsafe.Pointer(&dark)), 4)
	for _, h := range []uintptr{searchEdit, gameCombo, categoryCombo} {
		if h != 0 {
			pInvalidateRect.Call(h, 0, 1)
		}
	}
	if detail != nil {
		setDetailText(detail)
		applyDetailTheme()
	}
	invalidate()
}
func layout() {
	if hwndMain == 0 {
		return
	}
	var r RECT
	pGetClientRect.Call(hwndMain, uintptr(unsafe.Pointer(&r)))
	show := app.page < 3 && (app.page == 1 || app.authenticated)
	v := uintptr(SW_HIDE)
	if show {
		v = SW_SHOW
	}
	for _, h := range []uintptr{searchEdit, gameCombo, categoryCombo} {
		if h != 0 {
			pShowWindow.Call(h, v)
		}
	}
	if show {
		left := int32(252)
		width := r.Right - left - 32
		pSetWindowPos.Call(searchEdit, 0, uintptr(left), 218, uintptr(width-438), 36, SWP_NOZORDER|SWP_NOACTIVATE)
		pSetWindowPos.Call(gameCombo, 0, uintptr(r.Right-452), 218, 198, 300, SWP_NOZORDER|SWP_NOACTIVATE)
		pSetWindowPos.Call(categoryCombo, 0, uintptr(r.Right-242), 218, 210, 300, SWP_NOZORDER|SWP_NOACTIVATE)
	}
}
func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if r := recover(); r != nil {
			logError(fmt.Errorf("%v\n%s", r, debug.Stack()))
			win.Message(0, "Siri ModManager", "Der Fehler wurde in manager.log protokolliert.", 0x10)
		}
	}()
	if e := win.InitCOM(); e != nil {
		win.Message(0, "Siri ModManager", e.Error(), 0x10)
		return
	}
	defer win.CloseCOM()
	mutex, ok := win.SingleInstance()
	if !ok {
		win.Message(0, "Siri ModManager", "Der ModManager läuft bereits. Bitte das vorhandene Fenster öffnen.", 0x40)
		return
	}
	defer win.CloseHandle(mutex)
	createFonts()
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	_ = os.MkdirAll(win.AppDir(), 0755)
	if data, e := assets.ReadFile("app.ico"); e == nil {
		_ = os.WriteFile(appPath("app.ico"), data, 0644)
		iconHandle, _, _ = pLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr(appPath("app.ico")))), IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)
	}
	cls := ptr("SiriModManager07")
	wc := WNDCLASSEXW{CbSize: uint32(unsafe.Sizeof(WNDCLASSEXW{})), Style: CS_HREDRAW | CS_VREDRAW, LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hInst, HCursor: cursor, HIcon: iconHandle, HIconSm: iconHandle, LpszClassName: cls}
	if r, _, e := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		panic(e)
	}
	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	w, h := int32(1240), int32(820)
	if w > int32(sw)-40 {
		w = int32(sw) - 40
	}
	if h > int32(sh)-60 {
		h = int32(sh) - 60
	}
	hwndMain, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(ptr("Siri ModManager · "+core.Version))), WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN, uintptr((int32(sw)-w)/2), uintptr((int32(sh)-h)/2), uintptr(w), uintptr(h), 0, 0, hInst, 0)
	if hwndMain == 0 {
		panic("Fenster konnte nicht erstellt werden")
	}
	searchEdit, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr("EDIT"))), uintptr(unsafe.Pointer(ptr(""))), WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 0, 0, 100, 36, hwndMain, 1001, hInst, 0)
	pSendMessageW.Call(searchEdit, 0x1501, 1, uintptr(unsafe.Pointer(ptr("Mods suchen …"))))
	for j, out := range []*uintptr{&gameCombo, &categoryCombo} {
		*out, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr("COMBOBOX"))), 0, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|3, 0, 0, 200, 300, hwndMain, uintptr(1002+j), hInst, 0)
	}
	for _, s := range []string{"Alle Spiele", "Transport Fever 2", "Transport Fever 3"} {
		pSendMessageW.Call(gameCombo, cbAdd, 0, uintptr(unsafe.Pointer(ptr(s))))
	}
	pSendMessageW.Call(gameCombo, cbSetSel, 0, 0)
	for _, h := range []uintptr{searchEdit, gameCombo, categoryCombo} {
		pSendMessageW.Call(h, WM_SETFONT, fontRegular, 1)
	}
	fillCategories()
	updateTheme()
	layout()
	pSetTimer.Call(hwndMain, 2, 15*60*1000, 0)
	pShowWindow.Call(hwndMain, SW_SHOW)
	pUpdateWindow.Call(hwndMain)
	load()
	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if dispatchDetailMessage(&msg) {
			continue
		}
		if msg.Message == 0x100 && msg.WParam == 0x74 {
			refresh()
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
func wndProc(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		paint(hwnd)
		return 0
	case WM_SIZE:
		if hwndMain != 0 {
			layout()
			invalidate()
		}
		return 0
	case 0x14:
		return 1
	case wmMinMax:
		type minmax struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize POINT }
		m := (*minmax)(unsafe.Pointer(l))
		m.MinTrackSize = POINT{1000, 680}
		return 0
	case WM_LBUTTONDOWN:
		x, y := int32(int16(l)), int32(int16(l>>16))
		for _, b := range hits {
			if x >= b.r.Left && x <= b.r.Right && y >= b.r.Top && y <= b.r.Bottom {
				b.fn()
				return 0
			}
		}
		return 0
	case WM_COMMAND:
		id, notification := uint16(w), uint16(w>>16)
		if id == 1001 && notification == EN_CHANGE {
			app.query = getText(searchEdit)
			app.pager = 0
			invalidate()
		}
		if notification == 1 && (id == 1002 || id == 1003) {
			sel, _, _ := pSendMessageW.Call(l, cbSel, 0, 0)
			if id == 1002 {
				app.game = ""
				if sel == 1 {
					app.game = "TF2"
				}
				if sel == 2 {
					app.game = "TF3"
				}
			} else {
				app.category = ""
				if int(sel) > 0 && int(sel) < len(app.categories) {
					app.category = app.categories[sel]
				}
			}
			app.pager = 0
			invalidate()
		}
		return 0
	case WM_CTLCOLOREDIT, 0x0134, 0x0138:
		c := colors()
		pSetTextColor.Call(w, c.text)
		pSetBkColor.Call(w, c.panel)
		return editBrush
	case wmTasks:
		for {
			select {
			case fn := <-uiQueue:
				fn()
			default:
				return 0
			}
		}
	case WM_TIMER:
		if w == 2 && app.cfg.AutoCheck && !app.busy {
			refresh()
		}
		return 0
	case wmClose:
		if app.authPending && app.authCancel == nil {
			win.Message(hwnd, "Abmeldung läuft", "Die Abmeldung wird abgeschlossen. Bitte einen Moment warten.", 0x40)
			return 0
		}
		if app.busy {
			win.Message(hwnd, "Vorgang läuft", "Bitte den laufenden Vorgang abschließen lassen. Downloads kannst du unten mit „Abbrechen“ beenden.", 0x40)
			return 0
		}
		if app.authCancel != nil {
			app.authCancel()
		}
		closing.Store(true)
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		closing.Store(true)
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), w, l)
	return r
}
