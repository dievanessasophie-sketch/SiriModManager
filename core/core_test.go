package core

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRoot(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "mods")
	if e := os.Mkdir(p, 0755); e != nil {
		t.Fatal(e)
	}
	return p
}
func testZip(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, s := range files {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte(s))
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func testServer(t *testing.T, b []byte) (Mod, *httptest.Server) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	sum := sha256.Sum256(b)
	return Mod{ID: 1, Game: "TF2", Name: "Testmod", Version: "1.0", Folder: "siri_test_1", Download: s.URL, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(b))}, s
}
func testWrite(t *testing.T, p, s string) {
	os.MkdirAll(filepath.Dir(p), 0755)
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func TestInstallUpdateScanUninstall(t *testing.T) {
	r := testRoot(t)
	testWrite(t, filepath.Join(r, "other_mod_1", "mod.lua"), "other")
	m, s := testServer(t, testZip(t, map[string]string{"siri_test_1/mod.lua": "old", "siri_test_1/res/obsolete.txt": "old"}))
	defer s.Close()
	i, e := Install(context.Background(), s.Client(), m, r, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	list, e := Scan(r, "TF2", []Mod{m}, nil)
	if e != nil || len(list) != 2 {
		t.Fatalf("%v %v", list, e)
	}
	m2, s2 := testServer(t, testZip(t, map[string]string{"readme.txt": "Info", "wrapper/siri_new_2/mod.lua": "new", "wrapper/siri_new_2/res/new.txt": "new"}))
	defer s2.Close()
	m2.Version = "1.1"
	if !NeedsUpdate(m2, i) {
		t.Fatal("update missing")
	}
	i2, e := Install(context.Background(), s2.Client(), m2, r, &i, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(r, "siri_test_1")); !os.IsNotExist(e) {
		t.Fatal("old folder retained")
	}
	if _, e = os.Stat(filepath.Join(r, "siri_new_2", "res/new.txt")); e != nil {
		t.Fatal(e)
	}
	if e = Uninstall(i2); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(r, "other_mod_1/mod.lua")); e != nil {
		t.Fatal("unrelated mod touched")
	}
	list, e = Scan(r, "TF2", []Mod{m}, []Installed{i2})
	if e != nil || len(list) != 1 {
		t.Fatalf("stale install %v %v", list, e)
	}
}
func TestBadDownloadKeepsOldMod(t *testing.T) {
	for _, kind := range []string{"hash", "html", "traversal", "collision", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			r := testRoot(t)
			m, s := testServer(t, testZip(t, map[string]string{"siri_test_1/mod.lua": "old"}))
			defer s.Close()
			i, e := Install(context.Background(), s.Client(), m, r, nil, nil)
			if e != nil {
				t.Fatal(e)
			}
			files := map[string]string{"siri_test_1/mod.lua": "new"}
			if kind == "traversal" {
				files["../outside.txt"] = "bad"
			}
			if kind == "collision" {
				files = map[string]string{"other_mod_1/mod.lua": "bad"}
				testWrite(t, filepath.Join(r, "other_mod_1/mod.lua"), "other")
			}
			b := testZip(t, files)
			if kind == "html" {
				b = []byte("<html>Login</html>")
			}
			m2, s2 := testServer(t, b)
			defer s2.Close()
			if kind == "hash" {
				m2.SHA256 = strings.Repeat("0", 64)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			if _, e = Install(ctx, s2.Client(), m2, r, &i, nil); e == nil {
				t.Fatal("bad update accepted")
			}
			got, e := os.ReadFile(filepath.Join(r, "siri_test_1/mod.lua"))
			if e != nil || string(got) != "old" {
				t.Fatal("old mod lost")
			}
		})
	}
}
func TestBundlesAndSeparateGames(t *testing.T) {
	r2, r3 := testRoot(t), testRoot(t)
	m, s := testServer(t, testZip(t, map[string]string{"one_mod_1/mod.lua": "one", "two_mod_1/mod.lua": "two"}))
	defer s.Close()
	i, e := Install(context.Background(), s.Client(), m, r2, nil, nil)
	if e != nil || len(i.AllFolders()) != 2 {
		t.Fatalf("%v %v", i, e)
	}
	m3 := m
	m3.Game = "TF3"
	if _, e = Install(context.Background(), s.Client(), m3, r3, &i, nil); e == nil {
		t.Fatal("cross game accepted")
	}
	if _, e = Install(context.Background(), s.Client(), m3, r3, nil, nil); e != nil {
		t.Fatal(e)
	}
	a, _ := Scan(r2, "TF2", []Mod{m}, nil)
	b, _ := Scan(r3, "TF3", []Mod{m3}, nil)
	if len(a) != 1 || len(b) != 1 || a[0].Key() == b[0].Key() {
		t.Fatal("games mixed")
	}
	if e = Uninstall(i); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(r3, "one_mod_1")); e != nil {
		t.Fatal("TF3 deleted by TF2 uninstall")
	}
}
func TestRecoverInterruptedUpdate(t *testing.T) {
	r := testRoot(t)
	base, _ := txBase(r)
	tx := filepath.Join(base, "job-recovery")
	j := journal{Token: "job-recovery", Old: []string{"siri_old_1"}, New: []string{"siri_new_2"}, Game: "TF2", ID: 7}
	testWrite(t, filepath.Join(tx, "backup/siri_old_1/mod.lua"), "old")
	testWrite(t, filepath.Join(r, "siri_new_2/mod.lua"), "new")
	WriteJSON(filepath.Join(r, "siri_new_2", Marker), Installed{ID: 7, Game: "TF2", Transaction: j.Token})
	WriteJSON(filepath.Join(tx, "journal.json"), j)
	if e := Recover(r); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(r, "siri_old_1/mod.lua")); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(r, "siri_new_2")); !os.IsNotExist(e) {
		t.Fatal("partial mod survived")
	}
}
func TestSafePaths(t *testing.T) {
	for _, n := range []string{"..", "../x", "x\\y", "C:foo", "CON", "nul.txt", "x ", "x.", "foo:stream", ".hidden"} {
		if SafeName(n) {
			t.Errorf("accepted %q", n)
		}
	}
	r := testRoot(t)
	if ValidateRoot(r, r) == nil {
		t.Fatal("shared root")
	}
	for _, p := range []string{"/", filepath.Dir(r), "DEINEID/mods"} {
		if ValidateRoot(p, "") == nil {
			t.Errorf("bad root %s", p)
		}
	}
	if e := os.Symlink(t.TempDir(), filepath.Join(r, "linked_mod_1")); e == nil {
		if _, e = child(r, "linked_mod_1"); e == nil {
			t.Fatal("link accepted")
		}
	}
	if Uninstall(Installed{Root: r, Game: "TF2", Folders: []string{".."}}) == nil {
		t.Fatal("traversal uninstall accepted")
	}
}
func TestVersions(t *testing.T) {
	for _, v := range []struct {
		a, b string
		want bool
	}{{"1.10", "1.9", true}, {"v2.0.0", "1.9", true}, {"1.0.0", "1.0", false}, {"1.0-beta.2", "1.0", false}, {"1.0", "1.0-beta.2", true}, {"1.0-beta.10", "1.0-beta.2", true}, {"2.0", "unbekannt", false}, {"1.0", "2.0", false}} {
		if Newer(v.a, v.b) != v.want {
			t.Errorf("%s > %s", v.a, v.b)
		}
	}
}
func TestCatalogPagination(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !strings.Contains(r.URL.RawQuery, "siri-modbase-api/") {
			t.Error("route changed")
		}
		if r.URL.Query().Get("pageNo") == "2" {
			fmt.Fprint(w, `{"pages":2,"total":2,"mods":[{"id":"2","name":"Second","game":"tf3","download":"/second.zip"}],"items":[{"id":2,"category":"Gebäude"}]}`)
		} else {
			fmt.Fprint(w, `{"pages":2,"total":2,"mods":[{"id":1,"name":"First","game":"TF2"}],"items":[{"id":1,"category":"Assets"}]}`)
		}
	}))
	defer s.Close()
	list, e := FetchCatalog(context.Background(), s.Client(), s.URL+"/index.php?siri-modbase-api/")
	if e != nil || len(list) != 2 || calls != 2 {
		t.Fatalf("%+v %v", list, e)
	}
	if list[0].Game != "TF3" || list[0].Category != "Gebäude" || list[0].Download != s.URL+"/second.zip" {
		t.Fatalf("%+v", list[0])
	}
}
func TestInvalidCatalog(t *testing.T) {
	for _, b := range []string{" ", "<html>403</html>", `{"error":"forbidden"}`, `{"mods":[],"total":2}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, b) }))
		if _, e := FetchCatalog(context.Background(), s.Client(), s.URL); e == nil {
			t.Errorf("accepted %q", b)
		}
		s.Close()
	}
}
func TestPublicV2CatalogWithoutRelease(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		items := `[{"id":1,"title":"First","game":"tf2","category":"Assets","url":"/entry/1"}]`
		if r.URL.Query().Get("pageNo") == "2" {
			items = `[{"id":2,"title":"Second","game":"tf3","category":"Gebäude","url":"/entry/2"},{"id":3,"title":"No release","game":"tf2","category":"Stadtpakete","url":"/entry/3","versions":[]}]`
		}
		fmt.Fprintf(w, `{"schema":"siri-modbase-public-v2","pages":2,"total":3,"items":%s,"mods":[{"id":1,"name":"First","game":"TF2","version":"1.0","download":"/one.zip","detailURL":"/details/1"},{"id":2,"name":"Second","game":"TF3","version":"2.0","download":"/two.zip"}]}`, items)
	}))
	defer s.Close()
	list, err := FetchCatalog(context.Background(), s.Client(), s.URL+"/index.php?siri-modbase-api/")
	if err != nil || len(list) != 3 || calls != 2 {
		t.Fatalf("catalog=%+v calls=%d error=%v", list, calls, err)
	}
	if list[0].ID != 3 || list[0].CanInstall() || list[0].Version != "" || list[0].URL != s.URL+"/entry/3" {
		t.Fatalf("entry without release lost or made installable: %+v", list[0])
	}
	if !list[1].CanInstall() || list[1].Category != "Gebäude" || list[1].URL != s.URL+"/entry/2" {
		t.Fatalf("release metadata lost: %+v", list[1])
	}
	if list[2].URL != s.URL+"/details/1" || list[2].Download != s.URL+"/one.zip" {
		t.Fatalf("release detail/download URL lost: %+v", list[2])
	}
}
func TestPublicV2CatalogDetectsMissingEntries(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"schema":"siri-modbase-public-v2","pages":1,"total":2,"items":[{"id":1,"title":"First","game":"tf2"}],"mods":[{"id":1,"name":"First","game":"TF2"},{"id":2,"name":"Second","game":"TF2"}]}`)
	}))
	defer s.Close()
	if _, err := FetchCatalog(context.Background(), s.Client(), s.URL); err == nil {
		t.Fatal("accepted an incomplete public entry listing")
	}
}
func TestChangedRootDoesNotInheritVersion(t *testing.T) {
	a, b := testRoot(t), testRoot(t)
	testWrite(t, filepath.Join(b, "siri_test_1/mod.lua"), "local")
	list, e := Scan(b, "TF2", []Mod{{ID: 1, Game: "TF2", Name: "Test", Folder: "siri_test_1"}}, []Installed{{ID: 1, Game: "TF2", Version: "9.0", Root: a, Folder: "siri_test_1"}})
	if e != nil || len(list) != 1 || list[0].Version != "unbekannt" {
		t.Fatalf("%v %v", list, e)
	}
}
func TestAtomicJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := DefaultConfig()
	if e := WriteJSON(p, c); e != nil {
		t.Fatal(e)
	}
	c.Theme = "light"
	if e := WriteJSON(p, c); e != nil {
		t.Fatal(e)
	}
	var got Config
	if e := ReadJSON(p, &got); e != nil || got.Theme != "light" {
		t.Fatal("bad config")
	}
}
func TestUpcomingNotInstallable(t *testing.T) {
	if (Mod{ID: 1, Game: "TF3", Download: "https://example.com/mod.zip", Upcoming: true}).CanInstall() {
		t.Fatal("upcoming installable")
	}
}
