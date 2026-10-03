package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func SafeName(s string) bool {
	if s == "" || s == "." || s == ".." || len(s) > 180 || strings.HasPrefix(s, ".") || strings.TrimSpace(s) != s || strings.HasSuffix(s, ".") || strings.ContainsAny(s, "/\\:\x00<>\"|?*") {
		return false
	}
	for _, r := range s {
		if r < 32 {
			return false
		}
	}
	stem := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if regexp.MustCompile(`^(COM|LPT)[0-9]$`).MatchString(stem) {
		return false
	}
	return true
}
func rejectLinks(path string) error {
	p := filepath.Clean(path)
	for {
		st, e := os.Lstat(p)
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Verknüpfte Ordner werden nicht verändert: %s", p)
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return nil
}
func ValidateRoot(root, other string) error {
	if root == "" || !filepath.IsAbs(root) || strings.Contains(strings.ToUpper(root), "DEINEID") {
		return fmt.Errorf("Bitte einen vollständigen lokalen Mods-Ordner auswählen")
	}
	clean := filepath.Clean(root)
	if !strings.EqualFold(filepath.Base(clean), "mods") {
		return fmt.Errorf("Bitte den Unterordner „mods“ auswählen")
	}
	q := strings.ToLower(strings.ReplaceAll(clean, "\\", "/"))
	if strings.Contains(q, "/steamapps/workshop/") || strings.Contains(q, "/steamapps/common/transport fever 2/") {
		return fmt.Errorf("Bitte den persönlichen Mods-Ordner wählen, z. B. Steam/userdata/<ID>/1066780/local/mods")
	}
	if strings.HasPrefix(q, "//") || strings.HasPrefix(q, "\\\\") {
		return fmt.Errorf("Bitte einen lokalen Mods-Ordner auswählen")
	}
	if other != "" && strings.EqualFold(clean, filepath.Clean(other)) {
		return fmt.Errorf("TF2 und TF3 benötigen getrennte Mods-Ordner")
	}
	if e := rejectLinks(clean); e != nil {
		return e
	}
	st, e := os.Stat(clean)
	if e != nil {
		return e
	}
	if !st.IsDir() {
		return fmt.Errorf("Der Pfad ist kein Ordner")
	}
	return nil
}
func child(root, name string) (string, error) {
	if !SafeName(name) {
		return "", fmt.Errorf("Unsicherer Mod-Ordnername: %q", name)
	}
	p := filepath.Join(root, name)
	if e := rejectLinks(p); e != nil {
		return "", e
	}
	return p, nil
}
func validPackage(p, game string) bool {
	if st, e := os.Stat(filepath.Join(p, "mod.lua")); e == nil && !st.IsDir() {
		return true
	}
	if game == "TF3" {
		// Keep a TF3 package's metadata and content together. Otherwise the
		// recursive scan mistakes metadata/mod.lua for a standalone mod.
		if st, e := os.Stat(filepath.Join(p, "metadata", "mod.lua")); e == nil && !st.IsDir() {
			return true
		}
		for _, n := range []string{"mod.json", "manifest.json"} {
			if st, e := os.Stat(filepath.Join(p, n)); e == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}
func ReadMarker(path string) (Installed, error) {
	var i Installed
	e := ReadJSON(filepath.Join(path, Marker), &i)
	return i, e
}
func Scan(root, game string, mods []Mod, old []Installed) ([]Installed, error) {
	if root == "" || strings.Contains(strings.ToUpper(root), "DEINEID") {
		return nil, nil
	}
	if e := ValidateRoot(root, ""); e != nil {
		return nil, e
	}
	if e := Recover(root); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	found := map[string]Installed{}
	expected := map[string]int{}
	for _, d := range entries {
		if !d.IsDir() || !SafeName(d.Name()) {
			continue
		}
		p, e := child(root, d.Name())
		if e != nil {
			continue
		}
		i, e := ReadMarker(p)
		if e == nil && i.Game == game && i.ID > 0 {
			expected[i.Key()] = len(i.AllFolders())
			i.Root = root
			i.Folder = d.Name()
			i.Folders = []string{d.Name()}
			i.Managed = true
		} else {
			if !validPackage(p, game) {
				continue
			}
			i = Installed{Name: d.Name(), Game: game, Folder: d.Name(), Folders: []string{d.Name()}, Root: root, Version: "unbekannt"}
			for _, m := range mods {
				if m.Game == game && strings.EqualFold(m.Folder, d.Name()) {
					i.ID = m.ID
					i.Name = m.Name
					break
				}
			}
			// Import legacy records only if the exact configured root and folder still exist.
			for _, v := range old {
				if v.Game != game || (v.Root != "" && !strings.EqualFold(filepath.Clean(v.Root), filepath.Clean(root))) {
					continue
				}
				for _, folder := range v.AllFolders() {
					if strings.EqualFold(folder, d.Name()) {
						i.ID = v.ID
						i.Name = v.Name
						i.Version = v.Version
						i.SHA256 = v.SHA256
						i.Installed = v.Installed
						i.Managed = v.Managed
						break
					}
				}
			}
		}
		if prev, ok := found[i.Key()]; ok {
			prev.Folders = append(prev.Folders, d.Name())
			if prev.Version != i.Version {
				prev.Version = "unbekannt"
			}
			found[i.Key()] = prev
		} else {
			found[i.Key()] = i
		}
	}
	out := make([]Installed, 0, len(found))
	for _, i := range found {
		if expected[i.Key()] > len(i.AllFolders()) {
			i.Version = "unbekannt"
		}
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return strings.ToLower(out[a].Name) < strings.ToLower(out[b].Name) })
	return out, nil
}

// Detection is shallow and only considers documented TF2 userdata layouts.
func DetectTF2(roots []string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(p string) {
		if st, e := os.Stat(p); e == nil && st.IsDir() && !seen[strings.ToLower(p)] {
			out = append(out, p)
			seen[strings.ToLower(p)] = true
		}
	}
	for _, root := range roots {
		users, _ := os.ReadDir(filepath.Join(root, "userdata"))
		for _, u := range users {
			if !u.IsDir() {
				continue
			}
			p := filepath.Join(root, "userdata", u.Name(), "1066780", "local", "mods")
			add(p)
		}
	}
	if a := os.Getenv("APPDATA"); a != "" {
		add(filepath.Join(a, "Transport Fever 2", "mods"))
	}
	sort.Strings(out)
	return out
}
func treeNoLinks(path string) error {
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Modordner enthält eine Verknüpfung: %s", p)
		}
		return nil
	})
}
