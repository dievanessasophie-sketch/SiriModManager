package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Version = "0.8.0"
const APIURL = "https://forum.siri-mods.de/index.php?siri-modbase-api/"
const ModbaseURL = "https://forum.siri-mods.de/index.php?siri-modbase/"
const Marker = ".siri-modmanager.json"

type Mod struct {
	ID              int           `json:"id"`
	Name            string        `json:"name"`
	Game            string        `json:"game"`
	Version         string        `json:"version"`
	Author          string        `json:"author"`
	Description     string        `json:"description"`
	Category        string        `json:"category"`
	Image           string        `json:"image"`
	Download        string        `json:"download"`
	URL             string        `json:"url"`
	SHA256          string        `json:"sha256"`
	Folder          string        `json:"folder"`
	Changelog       string        `json:"changelog"`
	Size            int64         `json:"size"`
	Status          string        `json:"status"`
	Upcoming        bool          `json:"upcoming"`
	Features        string        `json:"features,omitempty"`
	MenuOptions     string        `json:"menuOptions,omitempty"`
	DownloadNote    string        `json:"downloadNote,omitempty"`
	FullDescription string        `json:"fullDescription,omitempty"`
	Installation    string        `json:"installation,omitempty"`
	Technical       string        `json:"technical,omitempty"`
	License         string        `json:"license,omitempty"`
	Tags            string        `json:"tags,omitempty"`
	Channel         string        `json:"channel,omitempty"`
	SteamURL        string        `json:"steamURL,omitempty"`
	SupportURL      string        `json:"supportURL,omitempty"`
	VideoURL        string        `json:"videoURL,omitempty"`
	UpdatedAt       string        `json:"updatedAt,omitempty"`
	CreatedAt       string        `json:"createdAt,omitempty"`
	Compatibility   string        `json:"compatibility,omitempty"`
	KnownIssues     string        `json:"knownIssues,omitempty"`
	Versions        []ReleaseInfo `json:"versions,omitempty"`
	Fields          []ExtraField  `json:"fields,omitempty"`
}

type ReleaseFile struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}
type ReleaseInfo struct {
	ID            int           `json:"id,omitempty"`
	Time          int64         `json:"time,omitempty"`
	Files         []ReleaseFile `json:"files,omitempty"`
	Version       string        `json:"version"`
	Channel       string        `json:"channel"`
	SteamURL      string        `json:"steamURL,omitempty"`
	PublishedAt   string        `json:"publishedAt,omitempty"`
	Changelog     string        `json:"changelog,omitempty"`
	Compatibility string        `json:"compatibility,omitempty"`
	KnownIssues   string        `json:"knownIssues,omitempty"`
}

type ExtraField struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

func (f *ExtraField) UnmarshalJSON(b []byte) error {
	var r map[string]json.RawMessage
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	f.Label, f.Type = textField(r, "label"), textField(r, "type")
	f.Value = textField(r, "value")
	if f.Value == "" {
		var v bool
		if json.Unmarshal(r["value"], &v) == nil && string(r["value"]) != "null" {
			if v {
				f.Value = "Ja"
			} else {
				f.Value = "Nein"
			}
		}
		var values []string
		if json.Unmarshal(r["value"], &values) == nil {
			f.Value = strings.Join(values, ", ")
		}
	}
	if f.Type == "boolean" {
		if f.Value == "1" {
			f.Value = "Ja"
		}
		if f.Value == "0" {
			f.Value = "Nein"
		}
	}
	return nil
}

func (m Mod) Key() string { return fmt.Sprintf("%s:%d", m.Game, m.ID) }
func (m Mod) CanInstall() bool {
	return m.ID > 0 && (m.Game == "TF2" || m.Game == "TF3") && m.Download != "" && !m.Upcoming
}
func NormalizeGame(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "_", "") {
	case "TF2", "TPF2", "TRANSPORTFEVER2", "1066780":
		return "TF2"
	case "TF3", "TPF3", "TRANSPORTFEVER3":
		return "TF3"
	}
	return s
}
func textField(raw map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				return strings.TrimSpace(strings.ReplaceAll(s, "\x00", ""))
			}
			var n json.Number
			if json.Unmarshal(v, &n) == nil {
				return n.String()
			}
			var obj map[string]json.RawMessage
			if json.Unmarshal(v, &obj) == nil {
				return textField(obj, "name", "title", "label")
			}
		}
	}
	return ""
}
func (m *Mod) UnmarshalJSON(b []byte) error {
	var r map[string]json.RawMessage
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*m = Mod{}
	m.ID, _ = strconv.Atoi(textField(r, "id", "entryID"))
	m.Name = textField(r, "name", "title")
	m.Game = NormalizeGame(textField(r, "game"))
	m.Version = textField(r, "version", "versionNumber")
	m.Author = textField(r, "author")
	m.Description = textField(r, "summary", "description")
	m.Category = textField(r, "category")
	m.Image = textField(r, "image", "imageURL")
	m.Download = textField(r, "download", "downloadURL")
	m.URL = textField(r, "url", "detailURL", "link")
	m.SHA256 = textField(r, "sha256")
	m.Folder = textField(r, "folder")
	m.Changelog = textField(r, "changelog")
	m.Size, _ = strconv.ParseInt(textField(r, "size"), 10, 64)
	m.Status = textField(r, "status")
	m.FullDescription = textField(r, "fullDescription", "description")
	m.Features = textField(r, "features")
	m.MenuOptions = textField(r, "menuOptions")
	m.Installation = textField(r, "installation")
	m.Technical = textField(r, "technical")
	m.License = textField(r, "license")
	m.Tags = textField(r, "tags")
	m.Channel = textField(r, "channel")
	m.SteamURL = textField(r, "steamURL")
	m.SupportURL = textField(r, "supportURL")
	m.VideoURL = textField(r, "videoURL")
	m.UpdatedAt = textField(r, "updatedAt")
	m.CreatedAt = textField(r, "createdAt")
	m.Compatibility = textField(r, "compatibility")
	m.KnownIssues = textField(r, "knownIssues")
	if len(r["versions"]) > 0 {
		if err := json.Unmarshal(r["versions"], &m.Versions); err != nil {
			return fmt.Errorf("Ungültiger Versionsverlauf: %w", err)
		}
	}
	if len(r["fields"]) > 0 {
		if err := json.Unmarshal(r["fields"], &m.Fields); err != nil {
			return fmt.Errorf("Ungültige Zusatzfelder: %w", err)
		}
	}
	// v3 returns releases with their protected file metadata instead of a flat download.
	if _, v3 := r["categoryID"]; v3 {
		m.Download = ""
		selected := -1
		for j, v := range m.Versions {
			if selected < 0 || (v.Channel == "stable" && m.Versions[selected].Channel != "stable") {
				selected = j
			}
			if v.Time > 0 {
				m.Versions[j].PublishedAt = time.Unix(v.Time, 0).UTC().Format(time.RFC3339)
			}
		}
		if selected >= 0 {
			v := m.Versions[selected]
			m.Version, m.Channel, m.Changelog = v.Version, v.Channel, v.Changelog
			m.Compatibility, m.KnownIssues, m.SteamURL = v.Compatibility, v.KnownIssues, v.SteamURL
			var zips []ReleaseFile
			for _, f := range v.Files {
				if strings.HasSuffix(strings.ToLower(f.Name), ".zip") {
					zips = append(zips, f)
				}
			}
			if len(zips) == 1 {
				f := zips[0]
				m.Download, m.SHA256, m.Size = f.URL, f.SHA256, f.Size
			} else if len(zips) > 1 {
				m.DownloadNote = "Mehrere ZIP-Dateien: Wähle die passende Variante in der ModBase."
			} else {
				m.DownloadNote = "Kein direkt installierbares ZIP-Paket. Öffne die ModBase für die verfügbaren Downloads."
			}
		} else {
			m.Upcoming = m.Status != "archived"
		}
	}
	_ = json.Unmarshal(r["upcoming"], &m.Upcoming)
	switch strings.ToLower(m.Status) {
	case "upcoming", "planned", "coming_soon", "announced":
		m.Upcoming = true
	}
	return nil
}

type Config struct {
	APIURL    string `json:"api_url"`
	TF2Path   string `json:"tf2_path"`
	TF3Path   string `json:"tf3_path"`
	Theme     string `json:"theme"`
	AutoCheck bool   `json:"auto_check"`
}

func (c Config) Root(game string) string {
	if game == "TF2" {
		return c.TF2Path
	}
	if game == "TF3" {
		return c.TF3Path
	}
	return ""
}
func DefaultConfig() Config { return Config{APIURL: APIURL, Theme: "dark", AutoCheck: true} }

type Installed struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Game        string    `json:"game"`
	Version     string    `json:"version"`
	Folder      string    `json:"folder,omitempty"`
	Folders     []string  `json:"folders,omitempty"`
	Root        string    `json:"root"`
	SHA256      string    `json:"sha256,omitempty"`
	Installed   time.Time `json:"installed"`
	Transaction string    `json:"transaction,omitempty"`
	Managed     bool      `json:"managed"`
}

func (i Installed) Key() string {
	if i.ID > 0 {
		return fmt.Sprintf("%s:%d", i.Game, i.ID)
	}
	return i.Game + ":local:" + strings.ToLower(i.Folder)
}
func (i Installed) AllFolders() []string {
	if len(i.Folders) > 0 {
		return i.Folders
	}
	if i.Folder != "" {
		return []string{i.Folder}
	}
	return nil
}
func VersionKnown(v string) bool { return v != "" && v != "unbekannt" }

var versionRE = regexp.MustCompile(`(?i)^v?(\d+(?:\.\d+)*)(?:[- ]?([a-z][a-z0-9.-]*))?(?:\+.*)?$`)

func Newer(remote, local string) bool {
	if !VersionKnown(local) || !VersionKnown(remote) {
		return false
	}
	a, b := versionRE.FindStringSubmatch(strings.TrimSpace(remote)), versionRE.FindStringSubmatch(strings.TrimSpace(local))
	if a == nil || b == nil {
		return false
	}
	aa, bb := strings.Split(a[1], "."), strings.Split(b[1], ".")
	n := len(aa)
	if len(bb) > n {
		n = len(bb)
	}
	for j := 0; j < n; j++ {
		x, y := 0, 0
		if j < len(aa) {
			x, _ = strconv.Atoi(aa[j])
		}
		if j < len(bb) {
			y, _ = strconv.Atoi(bb[j])
		}
		if x != y {
			return x > y
		}
	}
	// A final release is newer than its prerelease; numeric prerelease identifiers compare naturally.
	if a[2] == b[2] {
		return false
	}
	if a[2] == "" {
		return true
	}
	if b[2] == "" {
		return false
	}
	ta, tb := regexp.MustCompile(`[a-z]+|\d+`).FindAllString(strings.ToLower(a[2]), -1), regexp.MustCompile(`[a-z]+|\d+`).FindAllString(strings.ToLower(b[2]), -1)
	for j := 0; j < len(ta) && j < len(tb); j++ {
		if ta[j] == tb[j] {
			continue
		}
		x, ex := strconv.Atoi(ta[j])
		y, ey := strconv.Atoi(tb[j])
		if ex == nil && ey == nil {
			return x > y
		}
		return ta[j] > tb[j]
	}
	return len(ta) > len(tb)
}
func NeedsUpdate(m Mod, i Installed) bool {
	return m.CanInstall() && (Newer(m.Version, i.Version) || (m.Version == i.Version && m.SHA256 != "" && i.SHA256 != "" && !strings.EqualFold(m.SHA256, i.SHA256)))
}
func ReadJSON(path string, out any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
func WriteJSON(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".siri-json-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
