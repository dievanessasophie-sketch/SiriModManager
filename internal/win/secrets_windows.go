//go:build windows

package win

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var cryptProtect = crypt32.NewProc("CryptProtectData")
var cryptUnprotect = crypt32.NewProc("CryptUnprotectData")
var localFree = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")

// DPAPI defaults to the current Windows user; UI is forbidden. No machine-wide key.
func protect(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("Leere Anmeldedaten")
	}
	in := dataBlob{uint32(len(data)), &data[0]}
	var out dataBlob
	proc := cryptProtect
	if decrypt {
		proc = cryptUnprotect
	}
	ok, _, e := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(data)
	if ok == 0 {
		return nil, fmt.Errorf("Windows konnte die Anmeldedaten nicht schützen: %w", e)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	if out.Size > 1<<20 {
		return nil, fmt.Errorf("Ungültige Anmeldedaten")
	}
	result := append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...)
	return result, nil
}

type SecretStore struct{ Path string }

func (s SecretStore) Load() ([]byte, error) {
	b, e := os.ReadFile(s.Path)
	if e != nil {
		return nil, e
	}
	return protect(b, true)
}
func (s SecretStore) Delete() error {
	e := os.Remove(s.Path)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
func (s SecretStore) Save(b []byte) error {
	data, e := protect(b, false)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.Path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.Path), ".auth-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	// Windows replacement is atomic on the same volume; old file survives failed writes.
	from, _ := syscall.UTF16PtrFromString(tmp)
	to, _ := syscall.UTF16PtrFromString(s.Path)
	ok, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0x1|0x8)
	if ok == 0 {
		return e
	}
	return nil
}
