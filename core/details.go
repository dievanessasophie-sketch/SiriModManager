package core

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DetailSection struct{ Title, Text string }

var hiddenHTML = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
var blockHTML = regexp.MustCompile(`(?i)<(?:br\s*/?|/p|/div|/li|/h[1-6]|/tr)>`)
var listHTML = regexp.MustCompile(`(?i)<li\b[^>]*>`)
var bbFormat = regexp.MustCompile(`(?i)\[/?(?:b|i|u|s|color|size|font|align|center|left|right|url|quote|code|list|h[1-6])(?:=[^\]]*)?\]`)
var extraNewlines = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)

// DisplayText keeps paragraphs from Modbase text and renders no active markup.
func DisplayText(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = hiddenHTML.ReplaceAllString(s, "")
	s = listHTML.ReplaceAllString(s, "• ")
	s = blockHTML.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = bbFormat.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "[*]", "\n• ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(extraNewlines.ReplaceAllString(s, "\n\n"))
}

func ChannelLabel(s string) string {
	switch strings.ToLower(s) {
	case "stable":
		return "Stabil"
	case "beta":
		return "Beta"
	case "":
		return "Nicht angegeben"
	default:
		return DisplayText(s)
	}
}

func DisplayDate(s string) string {
	if n, e := strconv.ParseInt(s, 10, 64); e == nil && n > 0 {
		return time.Unix(n, 0).Local().Format("02.01.2006 · 15:04")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("02.01.2006 · 15:04")
	}
	return DisplayText(s)
}

func DetailSections(m Mod, installed *Installed) []DetailSection {
	var result []DetailSection
	add := func(title, body string) {
		if body = DisplayText(body); body != "" {
			result = append(result, DetailSection{title, body})
		}
	}
	description, heading := m.FullDescription, "Beschreibung"
	if description == "" {
		description, heading = m.Description, "Kurzbeschreibung"
	}
	if description == "" {
		description = "Für diesen Eintrag ist keine Beschreibung verfügbar."
	}
	add(heading, description)
	var data []string
	field := func(label, value string) {
		if value != "" {
			data = append(data, label+": "+value)
		}
	}
	game := m.Game
	if game == "TF2" {
		game = "Transport Fever 2"
	}
	if game == "TF3" {
		game = "Transport Fever 3"
	}
	field("Spiel", game)
	field("Autor", m.Author)
	field("Kategorie", m.Category)
	version := m.Version
	if version == "" {
		version = "Noch keine veröffentlichte Version"
	}
	field("Version", version)
	if m.Channel != "" {
		field("Veröffentlichung", ChannelLabel(m.Channel))
	}
	if m.Upcoming {
		field("Status", "Angekündigt")
	}
	if m.Size > 0 {
		field("Downloadgröße", fmt.Sprintf("%.1f MB", float64(m.Size)/1e6))
	}
	field("Eintrag erstellt", DisplayDate(m.CreatedAt))
	field("Zuletzt aktualisiert", DisplayDate(m.UpdatedAt))
	field("Schlagwörter", m.Tags)
	add("Mod-Daten", strings.Join(data, "\n"))
	add("Funktionen", m.Features)
	add("Menüoptionen", m.MenuOptions)
	add("Download", m.DownloadNote)
	add("Installation", m.Installation)
	add("Technische Angaben", m.Technical)
	add("Kompatibilität", m.Compatibility)
	add("Bekannte Probleme", m.KnownIssues)
	add("Lizenz und Nutzung", m.License)
	var custom []string
	for _, f := range m.Fields {
		if f.Label != "" && f.Value != "" {
			custom = append(custom, f.Label+"\n"+f.Value)
		}
	}
	add("Weitere Angaben aus der Modbase", strings.Join(custom, "\n\n"))
	add("Änderungsprotokoll", m.Changelog)
	var versions []string
	for _, v := range m.Versions {
		if v.Version == "" {
			continue
		}
		s := "Version " + v.Version
		if v.Channel != "" {
			s += " · " + ChannelLabel(v.Channel)
		}
		if v.PublishedAt != "" {
			s += "\n" + DisplayDate(v.PublishedAt)
		}
		if v.Compatibility != "" && v.Compatibility != m.Compatibility {
			s += "\nKompatibilität: " + v.Compatibility
		}
		if v.KnownIssues != "" && v.KnownIssues != m.KnownIssues {
			s += "\nBekannte Probleme: " + v.KnownIssues
		}
		if v.Changelog != "" && (v.Version != m.Version || DisplayText(v.Changelog) != DisplayText(m.Changelog)) {
			s += "\n" + v.Changelog
		}
		for _, f := range v.Files {
			s += "\nDatei: " + f.Name + fmt.Sprintf(" (%.1f MB)", float64(f.Size)/1e6)
		}
		versions = append(versions, s)
	}
	add("Versionsverlauf", strings.Join(versions, "\n\n"))
	if installed != nil {
		add("Auf deinem PC", "Installierte Version: "+installed.Version+"\nModordner: "+installed.Root+"\n"+strings.Join(installed.AllFolders(), "\n"))
	} else {
		add("Auf deinem PC", "In den ausgewählten Modordnern nicht als installiert erkannt.")
	}
	var links []string
	for _, link := range []struct{ name, url string }{{"Modbase", m.URL}, {"Steam Workshop", m.SteamURL}, {"Support", m.SupportURL}, {"Video", m.VideoURL}} {
		if link.url != "" {
			links = append(links, link.name+": "+link.url)
		}
	}
	add("Links", strings.Join(links, "\n\n"))
	return result
}
