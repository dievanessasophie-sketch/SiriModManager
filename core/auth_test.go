package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memorySecrets struct {
	data []byte
	fail bool
}

func (s *memorySecrets) Load() ([]byte, error) {
	if s.data == nil {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), s.data...), nil
}
func (s *memorySecrets) Save(b []byte) error {
	if s.fail {
		return fmt.Errorf("disk failure")
	}
	s.data = append([]byte(nil), b...)
	return nil
}
func (s *memorySecrets) Delete() error { s.data = nil; return nil }
func testTokens() TokenResponse {
	return TokenResponse{Access: randomToken(), Refresh: randomToken(), Type: "Bearer", Scope: nativeScope, ExpiresIn: 600, RefreshExpiresIn: 86400, UserID: 7, Username: "Freigeschaltet", DeviceID: 12}
}
func discoveryAt(base string) Discovery {
	return Discovery{Schema: "siri-modmanager-auth-v1", ClientID: nativeClient, Scope: nativeScope, Authorize: base + "/authorize", Token: base + "/token", Devices: base + "/devices", Verify: base + "/verify", Register: base + "/register", Modbase: base + "/mods"}
}
func authServer(t *testing.T, handler http.HandlerFunc) (*Auth, *httptest.Server, *memorySecrets) {
	t.Helper()
	s := httptest.NewTLSServer(handler)
	t.Cleanup(s.Close)
	store := &memorySecrets{}
	a, e := NewAuth(s.URL+"/api", store)
	if e != nil {
		t.Fatal(e)
	}
	a.http = s.Client()
	a.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	a.record.Discovery = discoveryAt(s.URL)
	return a, s, store
}
func authorizedClient(a *Auth, s *httptest.Server) *http.Client {
	c := a.Client(3 * time.Second)
	c.Transport = authTransport{a, s.Client().Transport}
	return c
}
func TestBrowserLoginPKCEAndCallback(t *testing.T) {
	var params atomic.Value
	var endpoint atomic.Value
	tok := testTokens()
	code := randomToken()
	a, s, store := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" {
			json.NewEncoder(w).Encode(discoveryAt(endpoint.Load().(string)))
			return
		}
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		p := params.Load().(url.Values)
		if r.Method != "POST" || r.Form.Get("code") != code || pkceChallenge(r.Form.Get("code_verifier")) != p.Get("code_challenge") || r.Form.Get("redirect_uri") != p.Get("redirect_uri") {
			t.Error("Code exchange lost PKCE or redirect binding")
			http.Error(w, "bad", 400)
			return
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("Browser cookie leaked")
		}
		json.NewEncoder(w).Encode(tok)
	})
	endpoint.Store(s.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e := a.Login(ctx, "Mein PC", func(raw string) error {
		u, e := url.Parse(raw)
		if e != nil {
			return e
		}
		p := u.Query()
		params.Store(p)
		if p.Get("code_challenge_method") != "S256" || p.Get("scope") != nativeScope || !tokenShape(p.Get("state")) {
			return fmt.Errorf("insecure authorization URL")
		}
		callback := p.Get("redirect_uri")
		res, e := http.Get(appendQuery(callback, url.Values{"state": {randomToken()}, "code": {code}}))
		if e != nil {
			return e
		}
		res.Body.Close()
		if res.StatusCode != 400 {
			return fmt.Errorf("wrong state accepted")
		}
		req, _ := http.NewRequest("GET", appendQuery(callback, url.Values{"state": {p.Get("state")}, "code": {code}}), nil)
		req.Host = "evil.example"
		res, e = http.DefaultClient.Do(req)
		if e != nil {
			return e
		}
		res.Body.Close()
		if res.StatusCode != 400 {
			return fmt.Errorf("host rebinding accepted")
		}
		res, e = http.Get(appendQuery(callback, url.Values{"state": {p.Get("state")}, "code": {code}}))
		if e != nil {
			return e
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("valid callback failed")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	name, signed, _ := a.Snapshot()
	if !signed || name != tok.Username || len(store.data) == 0 {
		t.Fatal("login not persisted")
	}
	restored, e := NewAuth(s.URL+"/api", store)
	if e != nil {
		t.Fatal(e)
	}
	_, signed, _ = restored.Snapshot()
	if !signed {
		t.Fatal("saved login not restored")
	}
	u, _ := url.Parse(params.Load().(url.Values).Get("redirect_uri"))
	client := http.Client{Timeout: 200 * time.Millisecond}
	if res, e := client.Get(u.String()); e == nil {
		res.Body.Close()
		t.Fatal("callback listener survived login")
	}
}
func TestPKCEKnownVector(t *testing.T) {
	if pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk") != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatal("incorrect S256")
	}
}
func TestParallelRequestsRotateOnce(t *testing.T) {
	old, next := testTokens(), testTokens()
	var refreshes atomic.Int32
	a, s, _ := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			r.ParseForm()
			if r.Form.Get("refresh_token") != old.Refresh {
				t.Error("wrong refresh token")
			}
			refreshes.Add(1)
			json.NewEncoder(w).Encode(next)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+next.Access {
			t.Error("wrong bearer")
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, "ok")
	})
	a.record.Token = old
	a.record.Expires = time.Now().Add(-time.Minute)
	client := authorizedClient(a, s)
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, e := client.Get(s.URL + "/file")
			if e != nil {
				t.Error(e)
				return
			}
			res.Body.Close()
		}()
	}
	wg.Wait()
	if refreshes.Load() != 1 {
		t.Fatalf("refresh count %d", refreshes.Load())
	}
}
func TestUnauthorizedRefreshAndRevocation(t *testing.T) {
	old, next := testTokens(), testTokens()
	var refreshes atomic.Int32
	var revoke atomic.Bool
	a, s, store := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			refreshes.Add(1)
			json.NewEncoder(w).Encode(next)
			return
		}
		if revoke.Load() {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"error":"verification_required"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+next.Access {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, "ok")
	})
	if e := a.saveLocked(old); e != nil {
		t.Fatal(e)
	}
	client := authorizedClient(a, s)
	res, e := client.Get(s.URL + "/file")
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if refreshes.Load() != 1 {
		t.Fatal("401 did not rotate")
	}
	revoke.Store(true)
	_, e = client.Get(s.URL + "/file")
	if !errors.Is(e, ErrVerificationRequired) {
		t.Fatalf("unexpected %v", e)
	}
	_, signed, _ := a.Snapshot()
	if signed || store.data != nil {
		t.Fatal("revoked login retained")
	}
}
func TestBearerNeverLeavesOriginOrWrites(t *testing.T) {
	var foreignRequests atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignRequests.Add(1) }))
	defer foreign.Close()
	a, s, _ := authServer(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, foreign.URL+"/steal", 302) })
	a.record.Token = testTokens()
	a.record.Expires = time.Now().Add(time.Hour)
	client := authorizedClient(a, s)
	for _, raw := range []string{s.URL + "/file", foreign.URL + "/file", "http://127.0.0.1:18000/file"} {
		res, e := client.Get(raw)
		if e == nil {
			res.Body.Close()
			t.Fatal("cross origin request accepted")
		}
	}
	req, _ := http.NewRequest("POST", s.URL+"/file", nil)
	if _, e := client.Do(req); e == nil {
		t.Fatal("write accepted")
	}
	if foreignRequests.Load() != 0 {
		t.Fatal("credentials reached foreign origin")
	}
}
func TestDiscoveryRejectsUntrustedEndpointsAndOldAPI(t *testing.T) {
	a, s, _ := authServer(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"schema":"siri-modbase-public-v2"}`) })
	if _, e := a.Discover(context.Background()); e == nil {
		t.Fatal("legacy public API accepted")
	}
	d := discoveryAt(s.URL)
	d.Token = "https://evil.test/token"
	if validDiscovery(s.URL, d) == nil {
		t.Fatal("foreign token endpoint accepted")
	}
	if _, e := NewAuth("http://forum.test/api", &memorySecrets{}); e == nil {
		t.Fatal("plaintext auth API accepted")
	}
}
func TestFailedRotationIsNotReplayed(t *testing.T) {
	var calls atomic.Int32
	a, s, store := authServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) })
	a.record.Token = testTokens()
	a.record.Expires = time.Now().Add(-time.Minute)
	client := authorizedClient(a, s)
	for j := 0; j < 2; j++ {
		if _, e := client.Get(s.URL + "/file"); e == nil {
			t.Fatal("failed token rotation accepted")
		}
	}
	if calls.Load() != 1 || store.data != nil {
		t.Fatal("ambiguous rotation was replayed")
	}
}
func TestLogoutAndSecureSaveFailureRevoke(t *testing.T) {
	var revokes atomic.Int32
	a, _, store := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "revoke" {
			t.Error("unexpected grant")
		}
		revokes.Add(1)
		fmt.Fprint(w, `{"revoked":true}`)
	})
	if e := a.saveLocked(testTokens()); e != nil {
		t.Fatal(e)
	}
	if e := a.Logout(context.Background()); e != nil {
		t.Fatal(e)
	}
	if store.data != nil {
		t.Fatal("logout retained secret")
	}
	store.fail = true
	if e := a.saveLocked(testTokens()); e == nil {
		t.Fatal("unprotected login accepted")
	}
	if revokes.Load() != 2 {
		t.Fatal("failed persistence did not revoke issued credential")
	}
}
func TestV3CatalogFromPlugin(t *testing.T) {
	b, e := os.ReadFile("testdata/manager_v3_fixture.json")
	if e != nil {
		t.Fatal(e)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer s.Close()
	mods, e := FetchCatalog(context.Background(), s.Client(), s.URL)
	if e != nil {
		t.Fatal(e)
	}
	if len(mods) != 1 || mods[0].Version != "1.2.3" || mods[0].Category != "Züge" || !mods[0].CanInstall() || mods[0].Game != "TF2" || !strings.Contains(mods[0].Download, "SiriModbaseApiFile") {
		t.Fatalf("v3 conversion: %#v", mods)
	}
	// Multiple variant archives must never silently install an arbitrary variant.
	var page catalogPage
	json.Unmarshal(b, &page)
	m := page.Mods[0]
	m.Versions[0].Files = append(m.Versions[0].Files, m.Versions[0].Files[0])
	raw := map[string]any{"id": m.ID, "name": m.Name, "game": "tf2", "categoryID": 1, "versions": m.Versions}
	j, _ := json.Marshal(raw)
	var multiple Mod
	json.Unmarshal(j, &multiple)
	if multiple.CanInstall() || multiple.DownloadNote == "" {
		t.Fatal("ambiguous archive auto-installed")
	}
}
func TestLoginCancellation(t *testing.T) {
	var base atomic.Value
	a, s, _ := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(discoveryAt(base.Load().(string)))
	})
	base.Store(s.URL)
	ctx, cancel := context.WithCancel(context.Background())
	e := a.Login(ctx, "PC", func(raw string) error { cancel(); return nil })
	if e == nil {
		t.Fatal("cancelled login succeeded")
	}
	_, signed, _ := a.Snapshot()
	if signed {
		t.Fatal("cancelled login persisted")
	}
}

// Ensure response bodies are usable by file installers after permission errors.
func TestFilePermissionErrorKeepsAccount(t *testing.T) {
	a, s, _ := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"error":"download_forbidden"}`)
	})
	a.record.Token = testTokens()
	a.record.Expires = time.Now().Add(time.Hour)
	res, e := authorizedClient(a, s).Get(s.URL + "/file")
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	_, signed, _ := a.Snapshot()
	if !signed || !strings.Contains(string(b), "download_forbidden") {
		t.Fatal("file-specific denial lost account or body")
	}
}
