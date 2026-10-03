package core

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxDownload int64 = 4 << 30
const maxExpanded int64 = 16 << 30
const txnDir = ".siri-manager-transactions"

type Progress func(string, int64, int64)
type progressWriter struct {
	ctx      context.Context
	w        io.Writer
	fn       Progress
	n, total int64
	last     time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	if e := p.ctx.Err(); e != nil {
		return 0, e
	}
	n, e := p.w.Write(b)
	p.n += int64(n)
	if p.fn != nil && time.Since(p.last) > 150*time.Millisecond {
		p.last = time.Now()
		p.fn("Download", p.n, p.total)
	}
	return n, e
}
func download(ctx context.Context, c *http.Client, m Mod, dest string, fn Progress) (string, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", m.Download, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	res, e := c.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("Download HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxDownload {
		return "", fmt.Errorf("Download überschreitet 4 GiB")
	}
	f, e := os.Create(dest)
	if e != nil {
		return "", e
	}
	h := sha256.New()
	p := &progressWriter{ctx: ctx, w: io.MultiWriter(f, h), fn: fn, total: res.ContentLength}
	_, e = io.Copy(p, io.LimitReader(res.Body, maxDownload+1))
	ce := f.Close()
	if e != nil {
		return "", e
	}
	if ce != nil {
		return "", ce
	}
	if p.n > maxDownload {
		return "", fmt.Errorf("Download überschreitet 4 GiB")
	}
	if res.ContentLength >= 0 && p.n != res.ContentLength {
		return "", fmt.Errorf("Download ist unvollständig")
	}
	if m.Size > 0 && m.Size != p.n {
		return "", fmt.Errorf("Dateigröße stimmt nicht mit der API überein")
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if m.SHA256 != "" && !strings.EqualFold(m.SHA256, sum) {
		return "", fmt.Errorf("SHA-256-Prüfung fehlgeschlagen – alter Mod bleibt erhalten")
	}
	return sum, nil
}
func unzip(ctx context.Context, archive, dest string) error {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return fmt.Errorf("Download ist kein gültiges ZIP-Archiv: %w", e)
	}
	defer z.Close()
	if len(z.File) > 100000 {
		return fmt.Errorf("Zu viele Dateien im Archiv")
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		if e := ctx.Err(); e != nil {
			return e
		}
		n := strings.ReplaceAll(f.Name, "\\", "/")
		n = strings.TrimSuffix(n, "/")
		if n == "" {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 || f.Mode()&os.ModeType != 0 && !f.FileInfo().IsDir() {
			return fmt.Errorf("Archiv enthält Verknüpfungen oder Spezialdateien")
		}
		parts := strings.Split(n, "/")
		for _, p := range parts {
			if !SafeName(p) {
				return fmt.Errorf("Unsicherer ZIP-Pfad: %s", f.Name)
			}
		}
		key := strings.ToLower(n)
		if seen[key] {
			return fmt.Errorf("Mehrfacher ZIP-Pfad: %s", n)
		}
		seen[key] = true
		// Archive-supplied manager metadata is never trusted.
		if strings.EqualFold(filepath.Base(n), Marker) {
			return fmt.Errorf("Archiv enthält reservierte Verwaltungsdaten")
		}
		total += f.UncompressedSize64
		if total > uint64(maxExpanded) {
			return fmt.Errorf("Entpackte Daten überschreiten 16 GiB")
		}
		p := filepath.Join(dest, filepath.FromSlash(n))
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(p, 0755); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e != nil {
			in.Close()
			return e
		}
		nbytes, e := io.Copy(out, io.LimitReader(in, int64(f.UncompressedSize64)+1))
		ce := out.Close()
		in.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if nbytes != int64(f.UncompressedSize64) {
			return fmt.Errorf("Fehlerhafte ZIP-Dateigröße")
		}
	}
	return nil
}

type packageDir struct{ Name, Path string }

func packages(root string, m Mod) ([]packageDir, error) {
	var candidates []string
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if len(strings.Split(rel, string(os.PathSeparator))) > 5 {
			return filepath.SkipDir
		}
		if validPackage(p, m.Game) {
			candidates = append(candidates, p)
			return filepath.SkipDir
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	if len(candidates) == 0 && m.Game == "TF3" {
		// TF3's native mod layout must be confirmed in game; preserve the supplied ZIP layout.
		es, e := os.ReadDir(root)
		if e != nil {
			return nil, e
		}
		if len(es) == 0 {
			return nil, fmt.Errorf("Leeres Archiv")
		}
		p := root
		if len(es) == 1 && es[0].IsDir() {
			p = filepath.Join(root, es[0].Name())
		}
		candidates = []string{p}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("Im ZIP fehlt mod.lua; kein installierbarer TF2-Mod")
	}
	if len(candidates) > 64 {
		return nil, fmt.Errorf("Zu viele Modordner in einem Paket")
	}
	out := []packageDir{}
	seen := map[string]bool{}
	for _, p := range candidates {
		name := filepath.Base(p)
		if p == root {
			name = m.Folder
		}
		if !SafeName(name) {
			if p == root {
				return nil, fmt.Errorf("Das ZIP enthält keinen übergeordneten Modordner. Die ModBase muss einen gültigen Zielordner liefern (folder), oder die ZIP-Datei muss wie der Modordner heißen, z. B. siri_dortmund_1.zip")
			}
			return nil, fmt.Errorf("Ungültiger Modordner im ZIP: %q", name)
		}
		if m.Game == "TF2" {
			parts := strings.Split(name, "_")
			number, numberErr := strconv.Atoi(parts[len(parts)-1])
			if len(parts) < 3 || numberErr != nil || number < 1 {
				return nil, fmt.Errorf("TF2-Modordner muss autor_name_nummer heißen: %s", name)
			}
			for _, r := range parts[len(parts)-1] {
				if r < '0' || r > '9' {
					return nil, fmt.Errorf("Ungültige TF2-Ordnerkennung: %s", name)
				}
			}
		}
		if seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("Doppelte Modordner im ZIP")
		}
		seen[strings.ToLower(name)] = true
		out = append(out, packageDir{name, p})
	}
	return out, nil
}

type journal struct {
	Token    string   `json:"token"`
	Old      []string `json:"old"`
	New      []string `json:"new"`
	Complete bool     `json:"complete"`
	Game     string   `json:"game"`
	ID       int      `json:"id"`
}

func txBase(root string) (string, error) {
	p := filepath.Join(root, txnDir)
	if e := rejectLinks(p); e != nil {
		return "", e
	}
	if e := os.MkdirAll(p, 0755); e != nil {
		return "", e
	}
	return p, nil
}
func rollback(root, tx string, j journal) error {
	// Remove only folders stamped by this transaction; never remove an unrelated collision.
	for _, n := range j.New {
		p, e := child(root, n)
		if e != nil {
			return e
		}
		i, e := ReadMarker(p)
		if e == nil && i.Transaction == j.Token && i.ID == j.ID && i.Game == j.Game {
			if e = treeNoLinks(p); e != nil {
				return e
			}
			if e = os.RemoveAll(p); e != nil {
				return e
			}
		}
	}
	for _, n := range j.Old {
		p, e := child(root, n)
		if e != nil {
			return e
		}
		b, e := child(filepath.Join(tx, "backup"), n)
		if e != nil {
			return e
		}
		if _, e = os.Stat(b); os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if _, e = os.Stat(p); !os.IsNotExist(e) {
			return fmt.Errorf("Wiederherstellung blockiert: %s existiert bereits", n)
		}
		if e = os.Rename(b, p); e != nil {
			return e
		}
	}
	return os.RemoveAll(tx)
}
func Recover(root string) error {
	base := filepath.Join(root, txnDir)
	if e := rejectLinks(base); e != nil {
		return e
	}
	es, e := os.ReadDir(base)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, d := range es {
		if !d.IsDir() || !strings.HasPrefix(d.Name(), "job-") || !SafeName(d.Name()) {
			continue
		}
		tx := filepath.Join(base, d.Name())
		if e = rejectLinks(tx); e != nil {
			return e
		}
		var j journal
		if e = ReadJSON(filepath.Join(tx, "journal.json"), &j); e != nil {
			continue
		}
		if j.Token != d.Name() {
			return fmt.Errorf("Ungültige Wiederherstellungsdaten")
		}
		for _, n := range append(append([]string{}, j.Old...), j.New...) {
			if !SafeName(n) {
				return fmt.Errorf("Ungültige Ordner in Wiederherstellungsdaten")
			}
		}
		if j.Complete {
			if e = treeNoLinks(tx); e != nil {
				return e
			}
			if e = os.RemoveAll(tx); e != nil {
				return e
			}
		} else {
			if e = rollback(root, tx, j); e != nil {
				return fmt.Errorf("Wiederherstellung: %w", e)
			}
		}
	}
	return nil
}
func Install(ctx context.Context, c *http.Client, m Mod, root string, old *Installed, fn Progress) (Installed, error) {
	var out Installed
	if !m.CanInstall() {
		return out, fmt.Errorf("Dieser Eintrag besitzt keinen installierbaren Direktdownload")
	}
	if e := ValidateRoot(root, ""); e != nil {
		return out, e
	}
	if old != nil && (old.Game != m.Game || old.ID != m.ID || !strings.EqualFold(filepath.Clean(old.Root), filepath.Clean(root))) {
		return out, fmt.Errorf("Installierter Mod gehört zu einem anderen Zielordner")
	}
	if e := Recover(root); e != nil {
		return out, e
	}
	base, e := txBase(root)
	if e != nil {
		return out, e
	}
	tx, e := os.MkdirTemp(base, "job-")
	if e != nil {
		return out, e
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(tx)
		}
	}()
	sum, e := download(ctx, c, m, filepath.Join(tx, "download.zip"), fn)
	if e != nil {
		return out, e
	}
	if fn != nil {
		fn("Archiv prüfen und entpacken", 0, 0)
	}
	unpacked := filepath.Join(tx, "unpacked")
	if e = unzip(ctx, filepath.Join(tx, "download.zip"), unpacked); e != nil {
		return out, e
	}
	pkgs, e := packages(unpacked, m)
	if e != nil {
		return out, e
	}
	j := journal{Token: filepath.Base(tx), Game: m.Game, ID: m.ID}
	oldset := map[string]bool{}
	if old != nil {
		for _, n := range old.AllFolders() {
			p, e := child(root, n)
			if e != nil {
				return out, e
			}
			if _, e = os.Stat(p); e == nil && old.Managed {
				current, e := ReadMarker(p)
				if e != nil || current.ID != old.ID || current.Game != old.Game {
					return out, fmt.Errorf("Modordner %s gehört nicht mehr zu diesem Eintrag", n)
				}
			}

			if !SafeName(n) {
				return out, fmt.Errorf("Ungültiger bisheriger Modordner")
			}
			j.Old = append(j.Old, n)
			oldset[strings.ToLower(n)] = true
		}
	}
	for _, p := range pkgs {
		target, e := child(root, p.Name)
		if e != nil {
			return out, e
		}
		if st, e := os.Stat(target); e == nil {
			if !st.IsDir() || !oldset[strings.ToLower(p.Name)] {
				return out, fmt.Errorf("Zielordner %s gehört nicht zu diesem verwalteten Mod", p.Name)
			}
		} else if !os.IsNotExist(e) {
			return out, e
		}
		j.New = append(j.New, p.Name)
	}
	sort.Strings(j.New)
	out = Installed{ID: m.ID, Name: m.Name, Game: m.Game, Version: m.Version, Folders: j.New, Folder: j.New[0], Root: root, SHA256: sum, Installed: time.Now(), Transaction: j.Token, Managed: true}
	if e = os.MkdirAll(filepath.Join(tx, "new"), 0755); e != nil {
		return out, e
	}
	if e = os.MkdirAll(filepath.Join(tx, "backup"), 0755); e != nil {
		return out, e
	}
	for _, p := range pkgs {
		if e = WriteJSON(filepath.Join(p.Path, Marker), out); e != nil {
			return out, e
		}
		if e = os.Rename(p.Path, filepath.Join(tx, "new", p.Name)); e != nil {
			return out, e
		}
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	if e = WriteJSON(filepath.Join(tx, "journal.json"), j); e != nil {
		return out, e
	}
	keep = true
	fail := func(err error) (Installed, error) {
		if re := rollback(root, tx, j); re != nil {
			return Installed{}, fmt.Errorf("%v; automatische Wiederherstellung fehlgeschlagen: %v", err, re)
		}
		return Installed{}, err
	}
	if fn != nil {
		fn("Modordner austauschen", 0, 0)
	}
	// After staging, commit is deliberately not cancellable. Every old folder is moved away first.
	for _, n := range j.Old {
		p, e := child(root, n)
		if e != nil {
			return fail(e)
		}
		if _, e = os.Stat(p); os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return fail(e)
		}
		if e = treeNoLinks(p); e != nil {
			return fail(e)
		}
		if e = os.Rename(p, filepath.Join(tx, "backup", n)); e != nil {
			return fail(fmt.Errorf("Mod wird verwendet oder ist schreibgeschützt: %w", e))
		}
	}
	for _, n := range j.New {
		p, e := child(root, n)
		if e != nil {
			return fail(e)
		}
		if e = os.Rename(filepath.Join(tx, "new", n), p); e != nil {
			return fail(e)
		}
	}
	j.Complete = true
	if e = WriteJSON(filepath.Join(tx, "journal.json"), j); e != nil {
		return fail(e)
	}
	if e = os.RemoveAll(tx); e != nil {
		if fn != nil {
			fn("Installiert; alte Sicherung wird beim nächsten Start bereinigt", 0, 0)
		}
	}
	return out, nil
}
func Uninstall(i Installed) error {
	if e := ValidateRoot(i.Root, ""); e != nil {
		return e
	}
	if len(i.AllFolders()) == 0 {
		return fmt.Errorf("Kein Modordner registriert")
	}
	for _, n := range i.AllFolders() {
		p, e := child(i.Root, n)
		if e != nil {
			return e
		}
		if _, e = os.Stat(p); os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if e = treeNoLinks(p); e != nil {
			return e
		}
		if i.Managed {
			current, e := ReadMarker(p)
			if e != nil || current.ID != i.ID || current.Game != i.Game {
				return fmt.Errorf("Modordner %s gehört nicht mehr zu diesem Eintrag", n)
			}
		}
	}
	if e := Recover(i.Root); e != nil {
		return e
	}
	base, e := txBase(i.Root)
	if e != nil {
		return e
	}
	tx, e := os.MkdirTemp(base, "job-")
	if e != nil {
		return e
	}
	j := journal{Token: filepath.Base(tx), Old: i.AllFolders(), Game: i.Game, ID: i.ID}
	if e = os.MkdirAll(filepath.Join(tx, "backup"), 0755); e != nil {
		os.RemoveAll(tx)
		return e
	}
	if e = WriteJSON(filepath.Join(tx, "journal.json"), j); e != nil {
		os.RemoveAll(tx)
		return e
	}
	fail := func(e error) error {
		if r := rollback(i.Root, tx, j); r != nil {
			return fmt.Errorf("%v; Wiederherstellung fehlgeschlagen: %v", e, r)
		}
		return e
	}
	for _, n := range j.Old {
		p, e := child(i.Root, n)
		if e != nil {
			return fail(e)
		}
		if _, e = os.Stat(p); os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return fail(e)
		}
		if e = os.Rename(p, filepath.Join(tx, "backup", n)); e != nil {
			return fail(e)
		}
	}
	j.Complete = true
	if e = WriteJSON(filepath.Join(tx, "journal.json"), j); e != nil {
		return fail(e)
	}
	if e = os.RemoveAll(tx); e != nil {
		return fmt.Errorf("Mod entfernt; Sicherung konnte noch nicht bereinigt werden: %w", e)
	}
	return nil
}
