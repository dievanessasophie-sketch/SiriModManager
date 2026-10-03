package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrLoginRequired = errors.New("Bitte im Forum anmelden und diesen PC verbinden.")
var ErrVerificationRequired = errors.New("Dein ModBase-Zugang ist nicht freigeschaltet oder wurde zurückgenommen.")

var ErrMFARequired = errors.New("Richte zuerst die im Forum erforderliche Zwei-Faktor-Anmeldung ein und verbinde diesen PC erneut.")

const nativeClient = "siri-modmanager"
const nativeScope = "modbase:read"

type Discovery struct {
	Schema    string `json:"schema"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	Authorize string `json:"authorization_endpoint"`
	Token     string `json:"token_endpoint"`
	Devices   string `json:"devices_url"`
	Verify    string `json:"verification_url"`
	Register  string `json:"register_url"`
	Modbase   string `json:"modbase_url"`
}
type TokenResponse struct {
	Access           string `json:"access_token"`
	Refresh          string `json:"refresh_token"`
	Type             string `json:"token_type"`
	Scope            string `json:"scope"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	UserID           int    `json:"user_id"`
	Username         string `json:"username"`
	DeviceID         int    `json:"device_id"`
}
type AuthRecord struct {
	APIURL    string        `json:"api_url"`
	Discovery Discovery     `json:"discovery"`
	Token     TokenResponse `json:"token"`
	Expires   time.Time     `json:"expires"`
}

// Save is called while the lock is held. Production implementations encrypt for the Windows user.
type SecretStore interface {
	Load() ([]byte, error)
	Save([]byte) error
	Delete() error
}
type Auth struct {
	mu     sync.Mutex
	record AuthRecord
	store  SecretStore
	http   *http.Client
}

func trustedURL(base, raw string) bool {
	b, e := url.Parse(base)
	if e != nil {
		return false
	}
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && b.Scheme == "https" && u.Host != "" && strings.EqualFold(u.Host, b.Host) && u.User == nil && u.Fragment == ""
}
func NewAuth(api string, store SecretStore) (*Auth, error) {
	if !trustedURL(api, api) {
		return nil, fmt.Errorf("Die Anmeldung benötigt eine HTTPS-API-Adresse ohne Zugangsdaten oder Fragment.")
	}
	a := &Auth{record: AuthRecord{APIURL: api}, store: store, http: authHTTP(25 * time.Second)}
	if b, e := store.Load(); e == nil {
		var r AuthRecord
		if json.Unmarshal(b, &r) == nil && r.APIURL == api && validDiscovery(api, r.Discovery) == nil && validTokens(r.Token) {
			a.record = r
		} else {
			if e = store.Delete(); e != nil {
				return nil, fmt.Errorf("Ungültige gespeicherte Anmeldung konnte nicht entfernt werden.")
			}
		}
	}
	return a, nil
}
func validDiscovery(api string, d Discovery) error {
	if d.Schema != "siri-modmanager-auth-v1" || d.ClientID != nativeClient || d.Scope != nativeScope {
		return fmt.Errorf("Das ModBase-Plugin benötigt das Update mit ModManager-Anmeldung (2.0 Beta 2).")
	}
	for _, u := range []string{d.Authorize, d.Token, d.Devices, d.Verify, d.Register, d.Modbase} {
		if !trustedURL(api, u) {
			return fmt.Errorf("Die API meldet eine unzulässige Anmeldeadresse.")
		}
	}
	return nil
}
func tokenShape(s string) bool {
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && len(s) == 43
}
func validTokens(t TokenResponse) bool {
	return tokenShape(t.Access) && tokenShape(t.Refresh) && t.Type == "Bearer" && t.Scope == nativeScope && t.UserID > 0 && t.DeviceID > 0 && t.ExpiresIn > 0 && t.ExpiresIn <= 3600 && t.RefreshExpiresIn > 0
}
func authHTTP(timeout time.Duration) *http.Client {
	c := Client(timeout)
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return c
}
func (a *Auth) Discover(ctx context.Context) (Discovery, error) {
	a.mu.Lock()
	api := a.record.APIURL
	a.mu.Unlock()
	u, _ := url.Parse(api)
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += "manager=discovery"
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	res, e := a.http.Do(req)
	if e != nil {
		return Discovery{}, fmt.Errorf("Anmeldeinformationen konnten nicht geladen werden. Verbindung und API-Adresse prüfen.")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Discovery{}, fmt.Errorf("Anmeldung nicht erreichbar (HTTP %d). ModBase-Plugin 2.0 Beta 2 und API-Aktivierung prüfen.", res.StatusCode)
	}
	var d Discovery
	if e = json.NewDecoder(io.LimitReader(res.Body, 32768)).Decode(&d); e != nil {
		return d, fmt.Errorf("Die API liefert keine Anmeldeinformationen. Bitte das ModBase-Plugin aktualisieren.")
	}
	if e = validDiscovery(api, d); e != nil {
		return d, e
	}
	a.mu.Lock()
	a.record.Discovery = d
	a.mu.Unlock()
	return d, nil
}
func (a *Auth) Snapshot() (string, bool, Discovery) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.record.Token.Username, a.record.Token.Refresh != "", a.record.Discovery
}
func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func pkceChallenge(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func appendQuery(raw string, v url.Values) string {
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + v.Encode()
}

// Login binds a single callback to loopback, PKCE, state, and a finite context deadline.
func (a *Auth) Login(ctx context.Context, device string, open func(string) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	d, e := a.Discover(ctx)
	if e != nil {
		return e
	}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return fmt.Errorf("Lokaler Anmeldeempfang konnte nicht geöffnet werden.")
	}
	defer ln.Close()
	redirect := "http://" + ln.Addr().String() + "/siri-modmanager/callback"
	state, verifier := randomToken(), randomToken()
	callback := make(chan string, 1)
	denied := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/siri-modmanager/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" || r.Host != ln.Addr().String() {
			http.Error(w, "Ungültige Anfrage", 400)
			return
		}
		q, e := url.ParseQuery(r.URL.RawQuery)
		if e != nil || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Diese Anmeldung gehört nicht zu diesem Manager.", 400)
			return
		}
		if q.Get("error") != "" {
			select {
			case denied <- struct{}{}:
			default:
			}
			fmt.Fprint(w, "Anmeldung abgebrochen. Du kannst dieses Fenster schließen.")
			return
		}
		if len(q["code"]) != 1 || !tokenShape(q.Get("code")) {
			http.Error(w, "Ungültiger Anmeldecode", 400)
			return
		}
		select {
		case callback <- q.Get("code"):
			fmt.Fprint(w, "Bestätigung empfangen. Wechsle zum Siri ModManager, um den Anmeldestatus zu sehen. Dieses Fenster kann geschlossen werden.")
		default:
			http.Error(w, "Bestätigung bereits empfangen", 409)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 8192}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ln) }()
	defer server.Close()
	if chars := []rune(device); len(chars) > 60 {
		device = string(chars[:60])
	}
	u := appendQuery(d.Authorize, url.Values{"client_id": {nativeClient}, "response_type": {"code"}, "scope": {nativeScope}, "redirect_uri": {redirect}, "state": {state}, "code_challenge": {pkceChallenge(verifier)}, "code_challenge_method": {"S256"}, "device_name": {device}})
	if e = open(u); e != nil {
		return fmt.Errorf("Der Browser konnte nicht geöffnet werden.")
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("Anmeldung abgebrochen oder nach fünf Minuten abgelaufen.")
	case <-denied:
		return fmt.Errorf("Die Freigabe wurde im Browser abgebrochen.")
	case <-serveErr:
		return fmt.Errorf("Der lokale Anmeldeempfang wurde beendet.")
	case code := <-callback:
		a.mu.Lock()
		defer a.mu.Unlock()
		t, e := a.tokenRequest(ctx, d.Token, url.Values{"grant_type": {"authorization_code"}, "client_id": {nativeClient}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
		if e != nil {
			return e
		}
		return a.saveLocked(t)
	}
}
func (a *Auth) tokenRequest(ctx context.Context, endpoint string, v url.Values) (TokenResponse, error) {
	var out TokenResponse
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	res, e := a.http.Do(req)
	if e != nil {
		return out, fmt.Errorf("Die Anmeldung konnte nicht bestätigt werden. Bitte erneut anmelden.")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 32768))
	if e != nil {
		return out, fmt.Errorf("Anmeldeantwort konnte nicht gelesen werden.")
	}
	var problem struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(b, &problem)
	if problem.Error == "mfa_required" {
		return out, ErrMFARequired
	}
	if problem.Error == "verification_required" {
		return out, ErrVerificationRequired
	}
	if res.StatusCode == 400 || res.StatusCode == 401 {
		return out, ErrLoginRequired
	}
	if res.StatusCode != 200 {
		return out, fmt.Errorf("Anmeldeserver antwortet mit HTTP %d.", res.StatusCode)
	}
	if v.Get("grant_type") == "revoke" {
		return out, nil
	}
	if json.Unmarshal(b, &out) != nil || !validTokens(out) {
		return out, fmt.Errorf("Ungültige Anmeldeantwort.")
	}
	return out, nil
}
func (a *Auth) clearLocked() error {
	a.record.Token = TokenResponse{}
	a.record.Expires = time.Time{}
	return a.store.Delete()
}
func (a *Auth) saveLocked(t TokenResponse) error {
	r := a.record
	r.Token = t
	r.Expires = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if e = a.store.Save(b); e != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = a.tokenRequest(ctx, r.Discovery.Token, url.Values{"grant_type": {"revoke"}, "client_id": {nativeClient}, "token": {t.Refresh}})
		_ = a.clearLocked()
		return fmt.Errorf("Die Anmeldung konnte nicht sicher in Windows gespeichert werden. Bitte erneut anmelden.")
	}
	a.record = r
	return nil
}
func (a *Auth) access(ctx context.Context, rejected string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.record.Token
	if t.Refresh == "" {
		return "", ErrLoginRequired
	}
	if time.Until(a.record.Expires) > 30*time.Second && (rejected == "" || rejected != t.Access) {
		return t.Access, nil
	}
	next, e := a.tokenRequest(ctx, a.record.Discovery.Token, url.Values{"grant_type": {"refresh_token"}, "client_id": {nativeClient}, "refresh_token": {t.Refresh}})
	if e != nil {
		// A lost response may already have rotated the token. Never retry a spent refresh token.
		_ = a.clearLocked()
		return "", errors.Join(ErrLoginRequired, e)
	}
	if next.UserID != t.UserID || next.DeviceID != t.DeviceID {
		_ = a.clearLocked()
		return "", ErrLoginRequired
	}
	if e = a.saveLocked(next); e != nil {
		return "", errors.Join(ErrLoginRequired, e)
	}
	return next.Access, nil
}
func (a *Auth) Logout(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.record.Token
	localErr := a.clearLocked()
	var remote error
	if t.Refresh != "" {
		_, remote = a.tokenRequest(ctx, a.record.Discovery.Token, url.Values{"grant_type": {"revoke"}, "client_id": {nativeClient}, "token": {t.Refresh}})
	}
	if localErr != nil {
		return fmt.Errorf("Die lokal gespeicherte Anmeldung konnte nicht entfernt werden.")
	}
	if remote != nil {
		return fmt.Errorf("Lokal abgemeldet. Der Server war nicht erreichbar; melde diesen PC zusätzlich unter „Geräte verwalten“ ab.")
	}
	return nil
}

type authTransport struct {
	auth *Auth
	base http.RoundTripper
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.auth.mu.Lock()
	api := t.auth.record.APIURL
	t.auth.mu.Unlock()
	if !trustedURL(api, req.URL.String()) {
		return nil, fmt.Errorf("Geschützte Dateien dürfen nur vom konfigurierten Forum geladen werden.")
	}
	if req.Method != "GET" && req.Method != "HEAD" {
		return nil, fmt.Errorf("Die Gerätefreigabe erlaubt nur lesende Abrufe.")
	}
	token, e := t.auth.access(req.Context(), "")
	if e != nil {
		return nil, e
	}
	for attempt := 0; attempt < 2; attempt++ {
		r := req.Clone(req.Context())
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Del("Cookie")
		res, e := t.base.RoundTrip(r)
		if e != nil {
			return nil, e
		}
		if res.StatusCode == 401 {
			res.Body.Close()
			if attempt == 1 {
				t.auth.mu.Lock()
				_ = t.auth.clearLocked()
				t.auth.mu.Unlock()
				return nil, ErrLoginRequired
			}
			token, e = t.auth.access(req.Context(), token)
			if e != nil {
				return nil, e
			}
			continue
		}
		if res.StatusCode == 403 {
			b, _ := io.ReadAll(io.LimitReader(res.Body, 32768))
			res.Body.Close()
			res.Body = io.NopCloser(bytes.NewReader(b))
			var problem struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(b, &problem)
			if problem.Error == "verification_required" || problem.Error == "mfa_required" {
				t.auth.mu.Lock()
				_ = t.auth.clearLocked()
				t.auth.mu.Unlock()
				if problem.Error == "mfa_required" {
					return nil, ErrMFARequired
				}
				return nil, ErrVerificationRequired
			}
		}
		return res, nil
	}
	return nil, ErrLoginRequired
}
func (a *Auth) Client(timeout time.Duration) *http.Client {
	c := Client(timeout)
	c.Transport = authTransport{a, c.Transport}
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("Zu viele API-Weiterleitungen.")
		}
		a.mu.Lock()
		api := a.record.APIURL
		a.mu.Unlock()
		if !trustedURL(api, r.URL.String()) {
			return fmt.Errorf("Weiterleitung geschützter Daten auf einen anderen Server blockiert.")
		}
		return nil
	}
	return c
}
