package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestManagerUpdate(t *testing.T) {
	for _, tc := range []struct {
		name, tag                     string
		status                        int
		draft, pre, setup, want, fail bool
	}{
		{name: "new", tag: "v0.9.0", status: 200, setup: true, want: true},
		{name: "equal", tag: "v0.8.1", status: 200, setup: true},
		{name: "old", tag: "v0.8.0", status: 200, setup: true},
		{name: "prerelease", tag: "v0.9.0", status: 200, pre: true, setup: true},
		{name: "draft", tag: "v0.9.0", status: 200, draft: true, setup: true},
		{name: "no installer", tag: "v0.9.0", status: 200},
		{name: "no release", status: 404},
		{name: "rate limit", status: 403, fail: true},
		{name: "invalid tag", tag: "../../other", status: 200, setup: true, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != managerLatestAPI || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Fatal("wrong destination or credentials")
				}
				assets := []any{}
				if tc.setup {
					assets = append(assets, map[string]string{"name": "SiriModManager_Setup.exe", "state": "uploaded"})
				}
				b, _ := json.Marshal(map[string]any{"tag_name": tc.tag, "draft": tc.draft, "prerelease": tc.pre, "assets": assets, "html_url": "https://untrusted.invalid"})
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header)}, nil
			})}
			update, err := CheckManagerUpdate(context.Background(), client, "0.8.1")
			if (err != nil) != tc.fail || update.Available != tc.want {
				t.Fatalf("%+v %v", update, err)
			}
			if update.Available && update.URL != ManagerReleasesURL+"/tag/"+tc.tag {
				t.Fatal("untrusted release URL")
			}
		})
	}
}
func TestManagerUpdateConfigMigration(t *testing.T) {
	cfg := DefaultConfig()
	if err := json.Unmarshal([]byte(`{"auto_check":false}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.ManagerAutoCheck || cfg.AutoCheck {
		t.Fatal("migration failed")
	}
	if err := json.Unmarshal([]byte(`{"manager_auto_check":false}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ManagerAutoCheck {
		t.Fatal("cannot disable")
	}
}
