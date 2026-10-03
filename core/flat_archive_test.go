package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFlatArchiveAPIInstallUpdateUninstall(t *testing.T) {
	for _, game := range []string{"TF2", "TF3"} {
		t.Run(game, func(t *testing.T) {
			root := testRoot(t)
			testWrite(t, filepath.Join(root, "other_mod_1", "mod.lua"), "keep")
			var installed *Installed
			for n := 0; n < 2; n++ {
				marker := "mod.lua"
				if game == "TF3" {
					marker = "metadata/mod.lua"
				}
				files := map[string]string{marker: "metadata", "res/current.txt": fmt.Sprint(n)}
				if n == 0 {
					files["res/obsolete.txt"] = "old"
				}
				original, server := testServer(t, testZip(t, files))
				defer server.Close()
				body, _ := json.Marshal(map[string]any{"id": 1, "name": "Flat mod", "game": game, "categoryID": 1, "status": "published", "versions": []any{map[string]any{"version": fmt.Sprintf("1.%d", n), "channel": "stable", "files": []ReleaseFile{{Name: "siri_flat_1.ZIP", URL: server.URL, Size: original.Size, SHA256: original.SHA256}}}}})
				var m Mod
				if err := json.Unmarshal(body, &m); err != nil {
					t.Fatal(err)
				}
				got, err := Install(context.Background(), server.Client(), m, root, installed, nil)
				if err != nil {
					t.Fatal(err)
				}
				installed = &got
				if got.Folder != "siri_flat_1" {
					t.Fatalf("wrong folder: %q", got.Folder)
				}
				data, err := os.ReadFile(filepath.Join(root, "siri_flat_1", "res/current.txt"))
				if err != nil || string(data) != fmt.Sprint(n) {
					t.Fatalf("wrong installed content: %s %v", data, err)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "siri_flat_1", "res/obsolete.txt")); !os.IsNotExist(err) {
				t.Fatal("obsolete file retained")
			}
			if _, err := os.Stat(filepath.Join(root, "siri_flat_1", "siri_flat_1")); !os.IsNotExist(err) {
				t.Fatal("double folder")
			}
			if err := Uninstall(*installed); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "other_mod_1", "mod.lua")); err != nil {
				t.Fatal("unrelated mod touched")
			}
		})
	}
}

func TestArchiveFolderMetadata(t *testing.T) {
	for _, tc := range []struct{ file, entryFolder, fileFolder, want string }{
		{"siri_test_1.zip", "", "", "siri_test_1"},
		{"download.zip", "siri_explicit_1", "", "siri_explicit_1"},
		{"download.zip", "", "siri_file_1", "siri_file_1"},
		{"siri_test_1.zip", "../bad", "", "../bad"}, // Never hide invalid explicit metadata.
		{"../siri_test_1.zip", "", "", ""},
		{`C:\siri_test_1.zip`, "", "", ""},
		{"Siri Test (2).zip", "", "", ""},
		{"siri_test_0.zip", "", "", ""},
		{"siri_test_1.zip.exe", "", "", ""},
	} {
		t.Run(tc.file+tc.entryFolder+tc.fileFolder, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"folder": tc.entryFolder, "categoryID": 1, "versions": []any{map[string]any{"files": []ReleaseFile{{Name: tc.file, Folder: tc.fileFolder}}}}})
			var m Mod
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m.Folder != tc.want {
				t.Fatalf("got %q want %q", m.Folder, tc.want)
			}
		})
	}
}
