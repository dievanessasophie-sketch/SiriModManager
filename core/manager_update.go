package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const ManagerReleasesURL = "https://github.com/dievanessasophie-sketch/SiriModManager/releases"
const managerLatestAPI = "https://api.github.com/repos/dievanessasophie-sketch/SiriModManager/releases/latest"

var managerStableTag = regexp.MustCompile(`^v?[0-9]{1,6}\.[0-9]{1,6}\.[0-9]{1,6}$`)

type ManagerUpdate struct {
	Version   string
	URL       string
	Available bool
}

// Use a separate, unauthenticated client: forum credentials must never reach GitHub.
func CheckManagerUpdate(ctx context.Context, client *http.Client, current string) (ManagerUpdate, error) {
	var result ManagerUpdate
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, managerLatestAPI, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("GitHub konnte nicht erreicht werden: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return result, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return result, fmt.Errorf("GitHub begrenzt die Anfragen. Bitte später erneut prüfen")
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("Update-Prüfung fehlgeschlagen (HTTP %d)", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return result, err
	}
	if len(b) > 1<<20 {
		return result, fmt.Errorf("Update-Antwort ist zu groß")
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(b, &release); err != nil {
		return result, fmt.Errorf("Ungültige Update-Antwort")
	}
	if release.Draft || release.Prerelease {
		return result, nil
	}
	if !managerStableTag.MatchString(release.Tag) {
		return result, fmt.Errorf("Unbekanntes Versionsformat der Veröffentlichung")
	}
	hasSetup := false
	for _, a := range release.Assets {
		if a.Name == "SiriModManager_Setup.exe" && a.State == "uploaded" {
			hasSetup = true
		}
	}
	if !hasSetup {
		return result, nil
	}
	result.Version = strings.TrimPrefix(release.Tag, "v")
	result.URL = ManagerReleasesURL + "/tag/" + release.Tag
	result.Available = Newer(result.Version, current)
	return result, nil
}
