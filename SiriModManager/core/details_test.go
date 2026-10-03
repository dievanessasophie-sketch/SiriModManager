package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestDetailMetadataAndCache(t *testing.T) {
	fixture := `{"schema":"siri-modbase-public-v2","total":1,"pages":1,"mods":[{"id":1,"name":"Test 🏠","game":"TF2","version":"2.0","description":"Kurztext","changelog":"Neue Fenster","channel":"stable","steamURL":"https://steamcommunity.com/sharedfiles/filedetails/?id=1","compatibility":"Ab Build 123"}],"items":[{"id":1,"title":"Test 🏠","game":"tf2","category":"Gebäude","url":"/entry/1","fullDescription":"<p>Ausführliche Beschreibung</p><p>Zweiter Absatz &amp; Details</p>","installation":"In den Mods-Ordner entpacken","technical":"LoD 0–3","license":"Private Nutzung","updatedAt":"2026-09-29T10:00:00Z","fields":[{"label":"Recolor","type":"boolean","value":true},{"label":"Terminals","type":"number","value":0}],"versions":[{"version":"2.0","channel":"stable","changelog":"Neue Fenster"},{"version":"1.0","channel":"beta","changelog":"Erste Version","publishedAt":"2026-09-20T10:00:00Z"}]}]}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, fixture) }))
	defer s.Close()
	mods, err := FetchCatalog(context.Background(), s.Client(), s.URL)
	if err != nil || len(mods) != 1 {
		t.Fatalf("%+v %v", mods, err)
	}
	m := mods[0]
	if len(m.Fields) != 2 || m.Fields[0].Value != "Ja" || m.Fields[1].Value != "0" || len(m.Versions) != 2 {
		t.Fatalf("details missing: %+v", m)
	}
	if m.FullDescription == "" || m.Installation == "" || m.License == "" || m.Technical == "" || m.UpdatedAt == "" || m.SteamURL == "" || m.Compatibility == "" {
		t.Fatalf("metadata missing: %+v", m)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var cached Mod
	if err = json.Unmarshal(b, &cached); err != nil || !reflect.DeepEqual(m, cached) {
		t.Fatalf("cache dropped detail data: %v\n%s", err, b)
	}
	var rendered []string
	for _, section := range DetailSections(cached, &Installed{Version: "1.0", Root: `C:\mods`, Folders: []string{"test_1"}}) {
		rendered = append(rendered, section.Title, section.Text)
	}
	text := strings.Join(rendered, "\n")
	for _, want := range []string{"Ausführliche Beschreibung\nZweiter Absatz & Details", "LoD 0–3", "Private Nutzung", "Recolor\nJa", "Terminals\n0", "Erste Version", "Installierte Version: 1.0", `C:\mods`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing detail %q in %s", want, text)
		}
	}
	if strings.Contains(text, "<p>") {
		t.Fatal("HTML leaked into detail text")
	}
}

func TestDisplayTextPreservesLongContent(t *testing.T) {
	long := strings.Repeat("Detaillierte Beschreibung. ", 4000)
	text := DisplayText("<script>hidden()</script>[b]Titel[/b]<br>" + long)
	if !strings.HasPrefix(text, "Titel\n") || !strings.Contains(text, strings.TrimSpace(long)) || strings.Contains(text, "hidden()") {
		t.Fatal("detail text truncated or active markup retained")
	}
}

func TestPreviewDecodeAndFailure(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 2, 1))
	im.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	im.SetRGBA(1, 0, color.RGBA{B: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			fmt.Fprint(w, "<html>Login</html>")
			return
		}
		w.Write(b.Bytes())
	}))
	defer s.Close()
	p, err := FetchPreview(context.Background(), s.Client(), s.URL)
	if err != nil || p.Width != 2 || p.Height != 1 || !bytes.Equal(p.BGRA, []byte{0, 0, 255, 255, 255, 0, 0, 255}) {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err = FetchPreview(context.Background(), s.Client(), s.URL+"/bad"); err == nil {
		t.Fatal("accepted HTML as a preview")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = FetchPreview(ctx, s.Client(), s.URL); err == nil {
		t.Fatal("ignored cancelled preview request")
	}
}
