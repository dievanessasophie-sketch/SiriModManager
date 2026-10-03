//go:build windows

package win

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")
var ole = syscall.NewLazyDLL("ole32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")

func Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", ""))
	return p
}
func Message(owner uintptr, title, text string, flags uintptr) uintptr {
	v, _, _ := user.NewProc("MessageBoxW").Call(owner, uintptr(unsafe.Pointer(Ptr(text))), uintptr(unsafe.Pointer(Ptr(title))), flags)
	return v
}
func Open(owner uintptr, path string) error {
	r, _, e := shell.NewProc("ShellExecuteW").Call(owner, uintptr(unsafe.Pointer(Ptr("open"))), uintptr(unsafe.Pointer(Ptr(path))), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("Öffnen fehlgeschlagen: %v", e)
	}
	return nil
}
func InitCOM() error {
	r, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(r) < 0 {
		return fmt.Errorf("COM 0x%x", r)
	}
	return nil
}
func CloseCOM() { ole.NewProc("CoUninitialize").Call() }

type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

func call(obj uintptr, index int, args ...uintptr) uintptr {
	v := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(v + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}
func release(obj uintptr) {
	if obj != 0 {
		call(obj, 2)
	}
}
func create(clsid, iid guid) (uintptr, error) {
	var obj uintptr
	r, _, _ := ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&obj)))
	if int32(r) < 0 {
		return 0, fmt.Errorf("Windows-Dialog 0x%x", r)
	}
	return obj, nil
}
func PickFolder(owner uintptr, title, initial string) (string, error) {
	clsid := guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iid := guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	obj, e := create(clsid, iid)
	if e != nil {
		return "", e
	}
	defer release(obj)
	var options uint32
	call(obj, 10, uintptr(unsafe.Pointer(&options)))
	r := call(obj, 9, uintptr(options|0x20|0x40|0x800|0x02000000))
	if int32(r) < 0 {
		return "", fmt.Errorf("Ordnerdialog 0x%x", r)
	}
	call(obj, 17, uintptr(unsafe.Pointer(Ptr(title))))
	if initial != "" {
		siid := guid{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
		var item uintptr
		r, _, _ := shell.NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(Ptr(initial))), 0, uintptr(unsafe.Pointer(&siid)), uintptr(unsafe.Pointer(&item)))
		if int32(r) >= 0 && item != 0 {
			call(obj, 12, item)
			release(item)
		}
	}
	r = call(obj, 3, owner)
	if uint32(r) == 0x800704c7 {
		return "", nil
	}
	if int32(r) < 0 {
		return "", fmt.Errorf("Ordnerauswahl 0x%x", r)
	}
	var item uintptr
	r = call(obj, 20, uintptr(unsafe.Pointer(&item)))
	if int32(r) < 0 || item == 0 {
		return "", fmt.Errorf("Kein Ordner ausgewählt")
	}
	defer release(item)
	var path *uint16
	r = call(item, 5, 0x80058000, uintptr(unsafe.Pointer(&path)))
	if int32(r) < 0 {
		return "", fmt.Errorf("Kein lokaler Ordner ausgewählt")
	}
	defer ole.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(path)))
	return readUTF16(path), nil
}
func readUTF16(p *uint16) string {
	if p == nil {
		return ""
	}
	b := []uint16{}
	for n := uintptr(0); n < 32768; n++ {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + n*2))
		if v == 0 {
			break
		}
		b = append(b, v)
	}
	return syscall.UTF16ToString(b)
}
func Shortcut(target, link, icon string) error {
	clsid := guid{0x00021401, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iid := guid{0x000214F9, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	obj, e := create(clsid, iid)
	if e != nil {
		return e
	}
	defer release(obj)
	if r := call(obj, 20, uintptr(unsafe.Pointer(Ptr(target)))); int32(r) < 0 {
		return fmt.Errorf("Verknüpfungsziel 0x%x", r)
	}
	call(obj, 9, uintptr(unsafe.Pointer(Ptr(filepath.Dir(target)))))
	call(obj, 17, uintptr(unsafe.Pointer(Ptr(icon))), 0)
	call(obj, 7, uintptr(unsafe.Pointer(Ptr("Siri ModManager – siri-mods.de"))))
	persistIID := guid{0x0000010B, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	var persist uintptr
	r := call(obj, 0, uintptr(unsafe.Pointer(&persistIID)), uintptr(unsafe.Pointer(&persist)))
	if int32(r) < 0 {
		return fmt.Errorf("Verknüpfung 0x%x", r)
	}
	defer release(persist)
	if e = os.MkdirAll(filepath.Dir(link), 0755); e != nil {
		return e
	}
	r = call(persist, 6, uintptr(unsafe.Pointer(Ptr(link))), 1)
	if int32(r) < 0 {
		return fmt.Errorf("Verknüpfung speichern 0x%x", r)
	}
	return nil
}
func KnownFolder(id int) (string, error) {
	buf := make([]uint16, 32768)
	r, _, _ := shell.NewProc("SHGetFolderPathW").Call(0, uintptr(id), 0, 0, uintptr(unsafe.Pointer(&buf[0])))
	if int32(r) < 0 {
		return "", fmt.Errorf("Windows-Ordner 0x%x", r)
	}
	return syscall.UTF16ToString(buf), nil
}
func DesktopShortcut(exe, icon string) error {
	d, e := KnownFolder(0x10)
	if e != nil {
		return e
	}
	return Shortcut(exe, filepath.Join(d, "Siri ModManager.lnk"), icon)
}
func SingleInstance() (uintptr, bool) {
	h, _, e := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(Ptr(`Local\SiriModManager`))))
	return h, e != syscall.ERROR_ALREADY_EXISTS && h != 0
}
func CloseHandle(h uintptr) { kernel.NewProc("CloseHandle").Call(h) }
func AppDir() string {
	if p := os.Getenv("SIRI_MODMANAGER_TEST_DATA"); p != "" {
		return p
	}
	p, _ := os.UserConfigDir()
	return filepath.Join(p, "SiriModManager")
}
