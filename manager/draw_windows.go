//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type BITMAPINFO struct{ Header BITMAPINFOHEADER }

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW  = user32.NewProc("RegisterClassExW")
	pCreateWindowExW   = user32.NewProc("CreateWindowExW")
	pDefWindowProcW    = user32.NewProc("DefWindowProcW")
	pShowWindow        = user32.NewProc("ShowWindow")
	pUpdateWindow      = user32.NewProc("UpdateWindow")
	pGetMessageW       = user32.NewProc("GetMessageW")
	pTranslateMessage  = user32.NewProc("TranslateMessage")
	pDispatchMessageW  = user32.NewProc("DispatchMessageW")
	pPostQuitMessage   = user32.NewProc("PostQuitMessage")
	pLoadCursorW       = user32.NewProc("LoadCursorW")
	pLoadImageW        = user32.NewProc("LoadImageW")
	pBeginPaint        = user32.NewProc("BeginPaint")
	pEndPaint          = user32.NewProc("EndPaint")
	pGetClientRect     = user32.NewProc("GetClientRect")
	pInvalidateRect    = user32.NewProc("InvalidateRect")
	pSetTimer          = user32.NewProc("SetTimer")
	pKillTimer         = user32.NewProc("KillTimer")
	pDestroyWindow     = user32.NewProc("DestroyWindow")
	pSetWindowPos      = user32.NewProc("SetWindowPos")
	pGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	pMessageBoxW       = user32.NewProc("MessageBoxW")
	pSendMessageW      = user32.NewProc("SendMessageW")
	pSetWindowTextW    = user32.NewProc("SetWindowTextW")
	pGetWindowTextW    = user32.NewProc("GetWindowTextW")
	pSetBkMode         = gdi32.NewProc("SetBkMode")
	pSetTextColor      = gdi32.NewProc("SetTextColor")
	pSetBkColor        = gdi32.NewProc("SetBkColor")
	pCreateSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	pCreatePen         = gdi32.NewProc("CreatePen")
	pSelectObject      = gdi32.NewProc("SelectObject")
	pDeleteObject      = gdi32.NewProc("DeleteObject")
	pRoundRect         = gdi32.NewProc("RoundRect")
	pRectangle         = gdi32.NewProc("Rectangle")
	pMoveToEx          = gdi32.NewProc("MoveToEx")
	pLineTo            = gdi32.NewProc("LineTo")
	pCreateFontW       = gdi32.NewProc("CreateFontW")
	pStretchDIBits     = gdi32.NewProc("StretchDIBits")
	pSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	pDrawTextW         = user32.NewProc("DrawTextW")
	pShellExecuteW     = shell32.NewProc("ShellExecuteW")
	pPostMessageW      = user32.NewProc("PostMessageW")
)

const (
	CS_HREDRAW          = 0x0002
	CS_VREDRAW          = 0x0001
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CLIPCHILDREN     = 0x02000000
	WS_POPUP            = 0x80000000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_BORDER           = 0x00800000
	WS_VSCROLL          = 0x00200000
	WS_TABSTOP          = 0x00010000
	ES_AUTOHSCROLL      = 0x0080
	SW_HIDE             = 0
	SW_SHOW             = 5
	SW_SHOWNORMAL       = 1
	WM_CREATE           = 0x0001
	WM_DESTROY          = 0x0002
	WM_PAINT            = 0x000F
	WM_SIZE             = 0x0005
	WM_TIMER            = 0x0113
	WM_COMMAND          = 0x0111
	WM_LBUTTONDOWN      = 0x0201
	WM_CTLCOLOREDIT     = 0x0133
	WM_SETFONT          = 0x0030
	WM_SETICON          = 0x0080
	WM_APP_REFRESH      = 0x8001
	EN_CHANGE           = 0x0300
	ICON_SMALL          = 0
	ICON_BIG            = 1
	IMAGE_ICON          = 1
	LR_LOADFROMFILE     = 0x0010
	LR_DEFAULTSIZE      = 0x0040
	COLOR_WINDOW        = 5
	IDC_ARROW           = 32512
	DT_LEFT             = 0x0000
	DT_CENTER           = 0x0001
	DT_RIGHT            = 0x0002
	DT_VCENTER          = 0x0004
	DT_SINGLELINE       = 0x0020
	DT_WORDBREAK        = 0x0010
	DT_END_ELLIPSIS     = 0x8000
	TRANSPARENT         = 1
	PS_SOLID            = 0
	NULL_PEN            = 8
	DIB_RGB_COLORS      = 0
	SRCCOPY             = 0x00CC0020
	HALFTONE            = 4
	MB_OK               = 0x00000000
	MB_ICONINFORMATION  = 0x00000040
	MB_YESNO            = 0x00000004
	MB_ICONQUESTION     = 0x00000020
	IDYES               = 6
	SWP_NOZORDER        = 0x0004
	SWP_NOACTIVATE      = 0x0010
)

var fontRegular, fontMedium, fontSemibold, fontBold, fontHero, fontLogo, editBrush uintptr

func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }
func cleanUIString(s string) string {
	// Win32 UTF-16 APIs treat NUL as a terminator. Strip embedded NULs from
	// server/user data so malformed metadata can never crash a paint callback.
	return strings.ReplaceAll(s, "\x00", "")
}

func ptr(s string) *uint16 {
	s = cleanUIString(s)
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil || p == nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func drawText(hdc uintptr, text string, r RECT, color uintptr, font uintptr, flags uint32) {
	text = cleanUIString(text)
	if text == "" {
		return
	}
	u, err := syscall.UTF16FromString(text)
	if err != nil || len(u) == 0 {
		return
	}
	pSelectObject.Call(hdc, font)
	pSetTextColor.Call(hdc, color)
	pSetBkMode.Call(hdc, TRANSPARENT)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), uintptr(flags))
}

func brush(color uintptr) uintptr { h, _, _ := pCreateSolidBrush.Call(color); return h }
func pen(color uintptr, width int32) uintptr {
	h, _, _ := pCreatePen.Call(PS_SOLID, uintptr(width), color)
	return h
}

func fillRound(hdc uintptr, r RECT, color uintptr, radius int32) {
	b := brush(color)
	if radius <= 0 {
		user32.NewProc("FillRect").Call(hdc, uintptr(unsafe.Pointer(&r)), b)
		pDeleteObject.Call(b)
		return
	}
	oldB, _, _ := pSelectObject.Call(hdc, b)
	nullPen, _, _ := gdi32.NewProc("GetStockObject").Call(NULL_PEN)
	oldP, _, _ := pSelectObject.Call(hdc, nullPen)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldP)
	pSelectObject.Call(hdc, oldB)
	pDeleteObject.Call(b)
}

func outlineRound(hdc uintptr, r RECT, color uintptr, width, radius int32) {
	p := pen(color, width)
	oldP, _, _ := pSelectObject.Call(hdc, p)
	nullBrush, _, _ := gdi32.NewProc("GetStockObject").Call(5) // NULL_BRUSH
	oldB, _, _ := pSelectObject.Call(hdc, nullBrush)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldB)
	pSelectObject.Call(hdc, oldP)
	pDeleteObject.Call(p)
}

func line(hdc uintptr, x1, y1, x2, y2 int32, color uintptr, width int32) {
	p := pen(color, width)
	old, _, _ := pSelectObject.Call(hdc, p)
	pMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	pLineTo.Call(hdc, uintptr(x2), uintptr(y2))
	pSelectObject.Call(hdc, old)
	pDeleteObject.Call(p)
}

func createFont(height int32, weight int32) uintptr {
	h, _, _ := pCreateFontW.Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(ptr("Segoe UI"))))
	return h
}

func createFonts() {
	fontRegular = createFont(-17, 400)
	fontMedium = createFont(-18, 500)
	fontSemibold = createFont(-18, 600)
	fontBold = createFont(-24, 700)
	fontHero = createFont(-38, 700)
	fontLogo = createFont(-23, 700)
}
