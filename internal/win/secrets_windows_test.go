//go:build windows

package win

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPISecretStore(t *testing.T) {
	s := SecretStore{Path: filepath.Join(t.TempDir(), "account.dpapi")}
	plain := []byte(`{"refresh_token":"local-test-secret"}`)
	if e := s.Save(plain); e != nil {
		t.Fatal(e)
	}
	disk, e := os.ReadFile(s.Path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(disk, plain) {
		t.Fatal("plaintext credential on disk")
	}
	got, e := s.Load()
	if e != nil || !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip %v", e)
	}
	changed := []byte(`{"refresh_token":"rotated"}`)
	if e = s.Save(changed); e != nil {
		t.Fatal(e)
	}
	got, e = s.Load()
	if e != nil || !bytes.Equal(got, changed) {
		t.Fatal("atomic replacement failed")
	}
	disk[len(disk)/2] ^= 1
	os.WriteFile(s.Path, disk, 0600)
	if _, e = s.Load(); e == nil {
		t.Fatal("tampered DPAPI accepted")
	}
	if e = s.Delete(); e != nil {
		t.Fatal(e)
	}
}
