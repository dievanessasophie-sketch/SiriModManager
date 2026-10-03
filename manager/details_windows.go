//go:build windows

package main

import (
	"context"
	"fmt"
	"sirimodmanager/core"
	"sirimodmanager/internal/win"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

type detailState struct {
	hwnd, body, modbaseButton, steamButton, closeButton uintptr
	mod                                                 core.Mod
	installed                                           *core.Installed
	preview                                             *core.Preview
	imageStatus                                         string
	cancel                                              context.CancelFunc
	imageGeneration                                     uint64
	offline                                             bool
}

var detail *detailState
var detailClassRegistered bool
var richEditDLL = syscall.NewLazyDLL("msftedit.dll")
var pIsDialogMessage = user32.NewProc("IsDialogMessageW")

const detailModbaseID = 2201
const detailSteamID = 2202
const detailCloseID = 2
const emSetCharFormat = 0x0444

type charFormat struct {
	Size, Mask, Effects uint32
	Height, Offset      int32
	Color               uint32
	CharSet, Pitch      byte
	Face                [32]uint16
}
type detailSpan struct{ from, to int }

func showDetails(m core.Mod, installed *core.Installed) {
	if err := richEditDLL.Load(); err != nil {
		fail(fmt.Errorf("Detailfenster konnte nicht geöffnet werden: %w", err))
		return
	}
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	class := ptr("SiriModManagerDetails072")
	if !detailClassRegistered {
		cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
		wc := WNDCLASSEXW{CbSize: uint32(unsafe.Sizeof(WNDCLASSEXW{})), Style: CS_HREDRAW | CS_VREDRAW, LpfnWndProc: syscall.NewCallback(detailWndProc), HInstance: hInst, HCursor: cursor, HIcon: iconHandle, HIconSm: iconHandle, LpszClassName: class}
		if r, _, e := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			fail(e)
			return
		}
		detailClassRegistered = true
	}
	if detail == nil {
		detail = &detailState{}
		var owner RECT
		user32.NewProc("GetWindowRect").Call(hwndMain, uintptr(unsafe.Pointer(&owner)))
		sw, _, _ := pGetSystemMetrics.Call(0)
		sh, _, _ := pGetSystemMetrics.Call(1)
		w, h := min(int32(960), int32(sw)-40), min(int32(800), int32(sh)-70)
		x, y := owner.Left+(owner.Right-owner.Left-w)/2, owner.Top+(owner.Bottom-owner.Top-h)/2
		detail.hwnd, _, _ = pCreateWindowExW.Call(0x00010000, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(ptr("Mod-Details · Siri ModManager"))), WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN, uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwndMain, 0, hInst, 0)
		if detail.hwnd == 0 {
			detail = nil
			fail(fmt.Errorf("Detailfenster konnte nicht erstellt werden"))
			return
		}
		detail.body, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr("RICHEDIT50W"))), 0, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|0x0004|0x0040|0x0800, 0, 0, 100, 100, detail.hwnd, 2200, hInst, 0)
		if detail.body == 0 {
			pDestroyWindow.Call(detail.hwnd)
			fail(fmt.Errorf("Textansicht konnte nicht erstellt werden"))
			return
		}
		pSendMessageW.Call(detail.body, 0x0435, 0, 16<<20) // EM_EXLIMITTEXT
		for _, b := range []struct {
			out   *uintptr
			id    uintptr
			title string
		}{{&detail.modbaseButton, detailModbaseID, "Modbase öffnen ↗"}, {&detail.steamButton, detailSteamID, "Steam Workshop ↗"}, {&detail.closeButton, detailCloseID, "Schließen"}} {
			*b.out, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr("BUTTON"))), uintptr(unsafe.Pointer(ptr(b.title))), WS_CHILD|WS_VISIBLE|WS_TABSTOP|0x000B, 0, 0, 100, 38, detail.hwnd, b.id, hInst, 0)
			pSendMessageW.Call(*b.out, WM_SETFONT, fontRegular, 1)
		}
	}
	d := detail
	d.mod, d.offline = m, !app.online
	d.installed = nil
	if installed != nil {
		copy := *installed
		d.installed = &copy
	}
	pSetWindowTextW.Call(d.hwnd, uintptr(unsafe.Pointer(ptr(m.Name+" · Mod-Details"))))
	setDetailText(d)
	applyDetailTheme()
	layoutDetails()
	startDetailPreview(d)
	pShowWindow.Call(d.hwnd, 9) // restore an already-open minimized detail window
	pUpdateWindow.Call(d.hwnd)
	user32.NewProc("SetForegroundWindow").Call(d.hwnd)
	user32.NewProc("SetFocus").Call(d.body)
}

func setDetailText(d *detailState) {
	var text strings.Builder
	var spans []detailSpan
	position := 0
	for _, section := range core.DetailSections(d.mod, d.installed) {
		title := cleanUIString(section.Title)
		body := strings.ReplaceAll(cleanUIString(section.Text), "\n", "\r")
		titleLength := len(utf16.Encode([]rune(title)))
		spans = append(spans, detailSpan{position, position + titleLength})
		block := title + "\r" + body + "\r\r\r"
		text.WriteString(block)
		position += len(utf16.Encode([]rune(block)))
	}
	pSendMessageW.Call(d.body, 0x000B, 0, 0) // WM_SETREDRAW
	pSetWindowTextW.Call(d.body, uintptr(unsafe.Pointer(ptr(text.String()))))
	c := colors()
	format := charFormat{Mask: 0xE0000001, Height: 240, Color: uint32(c.text)}
	format.Size = uint32(unsafe.Sizeof(format))
	copy(format.Face[:], utf16.Encode([]rune("Segoe UI")))
	pSendMessageW.Call(d.body, emSetCharFormat, 4, uintptr(unsafe.Pointer(&format))) // SCF_ALL
	format.Height, format.Effects, format.Color = 280, 1, uint32(c.accent)
	for _, span := range spans {
		pSendMessageW.Call(d.body, 0x00B1, uintptr(span.from), uintptr(span.to)) // EM_SETSEL
		pSendMessageW.Call(d.body, emSetCharFormat, 1, uintptr(unsafe.Pointer(&format)))
	}
	pSendMessageW.Call(d.body, 0x00B1, 0, 0)
	pSendMessageW.Call(d.body, 0x0115, 6, 0)       // vertical scroll to top
	pSendMessageW.Call(d.body, 0x0443, 0, c.panel) // EM_SETBKGNDCOLOR
	pSendMessageW.Call(d.body, 0x000B, 1, 0)
	pInvalidateRect.Call(d.body, 0, 1)
}

func applyDetailTheme() {
	if detail == nil || detail.hwnd == 0 {
		return
	}
	var dark int32 = 1
	if app.cfg.Theme == "light" {
		dark = 0
	}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute").Call(detail.hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	pInvalidateRect.Call(detail.hwnd, 0, 1)
	for _, h := range []uintptr{detail.modbaseButton, detail.steamButton, detail.closeButton} {
		pInvalidateRect.Call(h, 0, 1)
	}
}

func layoutDetails() {
	if detail == nil || detail.hwnd == 0 || detail.body == 0 {
		return
	}
	d := detail
	var r RECT
	pGetClientRect.Call(d.hwnd, uintptr(unsafe.Pointer(&r)))
	if r.Right < 100 || r.Bottom < 300 {
		return
	}
	pSetWindowPos.Call(d.body, 0, 32, 226, uintptr(r.Right-64), uintptr(r.Bottom-308), SWP_NOZORDER|SWP_NOACTIVATE)
	var body RECT
	pGetClientRect.Call(d.body, uintptr(unsafe.Pointer(&body)))
	body.Left, body.Top, body.Right, body.Bottom = 8, 8, body.Right-12, body.Bottom-8
	pSendMessageW.Call(d.body, 0x00B3, 0, uintptr(unsafe.Pointer(&body))) // EM_SETRECT
	y := r.Bottom - 57
	pSetWindowPos.Call(d.modbaseButton, 0, 24, uintptr(y), 184, 36, SWP_NOZORDER|SWP_NOACTIVATE)
	pSetWindowPos.Call(d.steamButton, 0, 220, uintptr(y), 190, 36, SWP_NOZORDER|SWP_NOACTIVATE)
	pSetWindowPos.Call(d.closeButton, 0, uintptr(r.Right-144), uintptr(y), 120, 36, SWP_NOZORDER|SWP_NOACTIVATE)
	visible := uintptr(SW_HIDE)
	if d.mod.SteamURL != "" {
		visible = SW_SHOW
	}
	pShowWindow.Call(d.steamButton, visible)
	pInvalidateRect.Call(d.hwnd, 0, 0)
}

func startDetailPreview(d *detailState) {
	if d.cancel != nil {
		d.cancel()
	}
	d.preview = nil
	d.imageGeneration++
	generation := d.imageGeneration
	d.imageStatus = "Kein Vorschaubild"
	if d.mod.Image == "" {
		return
	}
	d.imageStatus = "Vorschaubild wird geladen …"
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	d.cancel = cancel
	raw := d.mod.Image
	if app.auth == nil || !app.authenticated {
		cancel()
		d.cancel = nil
		d.imageStatus = "Anmeldung erforderlich"
		return
	}
	client := accountClient(20 * time.Second)
	worker(func() {
		defer cancel()
		preview, err := core.FetchPreview(ctx, client, raw)
		post(func() {
			if detail != d || d.imageGeneration != generation {
				return
			}
			handleAuthError(err)
			if detail != d {
				return
			}
			d.cancel = nil
			if err != nil {
				d.imageStatus = "Vorschaubild nicht verfügbar"
				logError(err)
			} else {
				d.preview = preview
				d.imageStatus = ""
			}
			pInvalidateRect.Call(d.hwnd, 0, 0)
		})
	})
}

func paintDetails(hwnd uintptr) {
	var ps PAINTSTRUCT
	dc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if detail == nil {
		return
	}
	d := detail
	var r RECT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	c := colors()
	fillRound(dc, r, c.bg, 0)
	picture := RECT{24, 26, 264, 188}
	fillRound(dc, picture, c.panel, 12)
	if p := d.preview; p != nil && len(p.BGRA) > 0 {
		w, h := int32(224), int32(p.Height)*224/int32(p.Width)
		if h > 146 {
			h = 146
			w = int32(p.Width) * 146 / int32(p.Height)
		}
		x, y := picture.Left+(240-w)/2, picture.Top+(162-h)/2
		info := BITMAPINFO{Header: BITMAPINFOHEADER{Width: int32(p.Width), Height: -int32(p.Height), Planes: 1, BitCount: 32}}
		info.Header.Size = uint32(unsafe.Sizeof(info.Header))
		pSetStretchBltMode.Call(dc, HALFTONE)
		pStretchDIBits.Call(dc, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, uintptr(p.Width), uintptr(p.Height), uintptr(unsafe.Pointer(&p.BGRA[0])), uintptr(unsafe.Pointer(&info)), DIB_RGB_COLORS, SRCCOPY)
	} else {
		drawText(dc, "S", RECT{44, 48, 244, 113}, c.accent, fontHero, DT_CENTER|DT_SINGLELINE)
		drawText(dc, d.imageStatus, RECT{40, 127, 248, 177}, c.muted, fontRegular, DT_CENTER|DT_WORDBREAK|0x0800)
	}
	drawText(dc, d.mod.Name, RECT{286, 25, r.Right - 24, 106}, c.text, fontBold, DT_LEFT|DT_WORDBREAK|0x0800)
	version := "Noch keine Version"
	if d.mod.Version != "" {
		version = "Version " + d.mod.Version
	}
	if d.mod.Upcoming {
		version = "Angekündigt"
	}
	drawText(dc, d.mod.Game+" · "+version, RECT{286, 116, r.Right - 24, 142}, c.accent, fontSemibold, DT_SINGLELINE|DT_END_ELLIPSIS|0x0800)
	drawText(dc, d.mod.Author+" · "+d.mod.Category, RECT{286, 148, r.Right - 24, 173}, c.muted, fontRegular, DT_SINGLELINE|DT_END_ELLIPSIS|0x0800)
	source := "Angaben aus der Siri Modbase"
	if d.mod.ID <= 0 {
		source = "Informationen zur lokalen Mod"
	} else if d.offline {
		source += " · Gespeicherter Stand"
	}
	drawText(dc, source, RECT{26, 193, r.Right - 24, 215}, c.muted, fontRegular, DT_SINGLELINE|0x0800)
	fillRound(dc, RECT{24, 220, r.Right - 24, r.Bottom - 76}, c.panel, 12)
	outlineRound(dc, RECT{24, 220, r.Right - 24, r.Bottom - 76}, c.line, 1, 12)
}

type detailDrawItem struct {
	Type, ID, Item, Action, State uint32
	Window, DC                    uintptr
	Rect                          RECT
	Data                          uintptr
}

func drawDetailButton(item *detailDrawItem) {
	c := colors()
	fillRound(item.DC, item.Rect, c.bg, 0)
	bg, fg := c.panel, c.text
	if item.ID == detailModbaseID {
		bg, fg = c.accent, c.accentText
	}
	fillRound(item.DC, item.Rect, bg, 10)
	outlineRound(item.DC, item.Rect, c.line, 1, 10)
	drawText(item.DC, getText(item.Window), item.Rect, fg, fontSemibold, DT_CENTER|DT_VCENTER|DT_SINGLELINE|0x0800)
	if item.State&0x0010 != 0 {
		r := item.Rect
		r.Left += 3
		r.Top += 3
		r.Right -= 3
		r.Bottom -= 3
		outlineRound(item.DC, r, fg, 1, 8)
	}
}

func dispatchDetailMessage(msg *MSG) bool {
	if detail == nil {
		return false
	}
	inside, _, _ := user32.NewProc("IsChild").Call(detail.hwnd, msg.Hwnd)
	if msg.Hwnd != detail.hwnd && inside == 0 {
		return false
	}
	if msg.Message == 0x0100 && msg.WParam == 0x1B {
		pDestroyWindow.Call(detail.hwnd)
		return true
	}
	r, _, _ := pIsDialogMessage.Call(detail.hwnd, uintptr(unsafe.Pointer(msg)))
	return r != 0
}

func detailWndProc(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		paintDetails(hwnd)
		return 0
	case WM_SIZE:
		layoutDetails()
		return 0
	case 0x0014:
		return 1
	case wmMinMax:
		type minmax struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize POINT }
		(*minmax)(unsafe.Pointer(l)).MinTrackSize = POINT{660, 540}
		return 0
	case 0x002B:
		if l != 0 {
			drawDetailButton((*detailDrawItem)(unsafe.Pointer(l)))
			return 1
		}
	case WM_COMMAND:
		if detail == nil {
			return 0
		}
		var url string
		switch uint16(w) {
		case detailCloseID:
			pDestroyWindow.Call(hwnd)
			return 0
		case detailModbaseID:
			url = detail.mod.URL
			if url == "" {
				url = core.ModbaseURL
			}
		case detailSteamID:
			url = detail.mod.SteamURL
		}
		if url != "" {
			if err := win.Open(hwnd, url); err != nil {
				win.Message(hwnd, "Link öffnen", err.Error(), 0x10)
			}
		}
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		if detail != nil && detail.hwnd == hwnd {
			if detail.cancel != nil {
				detail.cancel()
			}
			detail = nil
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), w, l)
	return r
}
