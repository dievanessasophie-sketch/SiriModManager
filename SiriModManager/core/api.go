package core

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("Zu viele Weiterleitungen")
		}
		if via[0].URL.Scheme == "https" && r.URL.Scheme != "https" {
			return fmt.Errorf("Unsichere Download-Weiterleitung")
		}
		return nil
	}}
}
func validURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") && u.User == nil
}
func resolveURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	b, e := url.Parse(base)
	if e != nil {
		return ""
	}
	r, e := url.Parse(ref)
	if e != nil {
		return ""
	}
	v := b.ResolveReference(r).String()
	if !validURL(v) {
		return ""
	}
	return v
}
func PageURL(raw string, n int) string {
	// Preserve WoltLab's unescaped route key; only replace the pagination argument.
	u, e := url.Parse(raw)
	if e != nil {
		return raw
	}
	parts := []string{}
	for _, p := range strings.Split(u.RawQuery, "&") {
		if p != "" && !strings.HasPrefix(p, "pageNo=") {
			parts = append(parts, p)
		}
	}
	parts = append(parts, "pageNo="+strconv.Itoa(n))
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}

type catalogPage struct {
	Schema string `json:"schema"`
	Pages  int    `json:"pages"`
	Total  int    `json:"total"`
	Mods   []Mod  `json:"mods"`
	Data   []Mod  `json:"data"`
	Items  []Mod  `json:"items"`
}

func fetchPage(ctx context.Context, c *http.Client, raw string) (catalogPage, error) {
	var out catalogPage
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return out, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	res, e := c.Do(req)
	if e != nil {
		return out, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return out, fmt.Errorf("API HTTP %d – Anmeldung, Freischaltung und API-Aktivierung prüfen", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (16<<20)+1))
	if e != nil {
		return out, e
	}
	if len(b) > 16<<20 {
		return out, fmt.Errorf("API-Antwort ist zu groß")
	}
	if len(strings.TrimSpace(string(b))) > 0 && strings.TrimSpace(string(b))[0] == '[' {
		e = json.Unmarshal(b, &out.Mods)
	} else {
		e = json.Unmarshal(b, &out)
	}
	if e != nil {
		return out, fmt.Errorf("API liefert kein gültiges Mod-JSON: %w", e)
	}
	if out.Mods == nil && out.Data == nil && out.Items == nil {
		return out, fmt.Errorf("API-Schema ohne mods, data oder items")
	}
	return out, nil
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func plain(s string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(s, " ")))
}
func FetchCatalog(ctx context.Context, c *http.Client, raw string) ([]Mod, error) {
	if !validURL(raw) {
		return nil, fmt.Errorf("Ungültige API-Adresse")
	}
	first, e := fetchPage(ctx, c, raw)
	if e != nil {
		return nil, e
	}
	if first.Pages > 200 {
		return nil, fmt.Errorf("API meldet zu viele Seiten")
	}
	records := map[string]Mod{}
	entries := map[string]Mod{}
	categories := map[int]string{}
	add := func(p catalogPage) {
		for _, v := range p.Items {
			if v.Category != "" {
				categories[v.ID] = v.Category
			}
			if v.ID > 0 && v.Name != "" {
				entries[v.Key()] = v
			}
		}
		part := p.Mods
		if part == nil {
			part = p.Data
		}
		if part == nil {
			part = p.Items
		}
		for _, m := range part {
			if m.ID <= 0 || m.Name == "" {
				continue
			}
			if old, ok := records[m.Key()]; ok && m.Category == "" {
				m.Category = old.Category
			}
			records[m.Key()] = m
		}
	}
	add(first)
	for n := 2; n <= first.Pages; n++ {
		p, e := fetchPage(ctx, c, PageURL(raw, n))
		if e != nil {
			return nil, fmt.Errorf("Seite %d: %w", n, e)
		}
		add(p)
	}
	if first.Schema == "siri-modbase-public-v2" {
		// Public v2 paginates all published entries in items. Its mods array
		// repeats the complete release list and may omit entries without a release.
		if first.Total > len(entries) {
			return nil, fmt.Errorf("Unvollständiger Katalog: %d von %d Einträgen", len(entries), first.Total)
		}
		for key, entry := range entries {
			if m, ok := records[key]; ok {
				if m.URL == "" {
					m.URL = entry.URL
				}
				if m.Category == "" {
					m.Category = entry.Category
				}
				m.FullDescription = entry.FullDescription
				m.Installation, m.Technical, m.License = entry.Installation, entry.Technical, entry.License
				m.Tags, m.SupportURL, m.VideoURL = entry.Tags, entry.SupportURL, entry.VideoURL
				m.UpdatedAt, m.CreatedAt = entry.UpdatedAt, entry.CreatedAt
				m.Versions, m.Fields = entry.Versions, entry.Fields
				records[key] = m
			} else {
				// Entry metadata alone is not an installable release.
				entry.Download = ""
				entry.Version = ""
				records[key] = entry
			}
		}
	}
	result := make([]Mod, 0, len(records))
	for _, m := range records {
		if m.Category == "" {
			m.Category = categories[m.ID]
		}
		if m.Category == "" {
			m.Category = "Sonstige"
		}
		m.Name = plain(m.Name)
		m.Description = plain(m.Description)
		m.Changelog = DisplayText(m.Changelog)
		m.Download = resolveURL(raw, m.Download)
		m.Image = resolveURL(raw, m.Image)
		m.URL = resolveURL(raw, m.URL)
		m.SteamURL = resolveURL(raw, m.SteamURL)
		m.SupportURL = resolveURL(raw, m.SupportURL)
		m.VideoURL = resolveURL(raw, m.VideoURL)
		for j := range m.Versions {
			m.Versions[j].SteamURL = resolveURL(raw, m.Versions[j].SteamURL)
		}
		if m.URL == "" {
			m.URL = ModbaseURL
		}
		result = append(result, m)
	}
	if first.Total > len(result) {
		return nil, fmt.Errorf("Unvollständiger Katalog: %d von %d Einträgen", len(result), first.Total)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result, nil
}
