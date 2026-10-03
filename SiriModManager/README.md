# Siri ModManager 0.8.0

Windows 10/11, 64 Bit. Native Anwendung für siri-mods.de mit Desktop-Icon.
Keine zusätzliche Laufzeitumgebung und keine Administratorrechte nötig.

Quellcode-Lizenz: **MIT**, siehe [LICENSE](LICENSE). Die enthaltenen Programmdateien
und das Icon dürfen unter dieser Lizenz weiterverwendet werden. Über die ModBase
heruntergeladene Mods sind eigene Inhalte und nicht Bestandteil dieses Repositories.

**Code signing policy:** [Signierregeln und aktueller Status](CODE_SIGNING.md).
Die SignPath-Bewerbung ist vorbereitet; eine Aufnahme oder bereits erfolgte
Signierung wird damit nicht behauptet. [Datenschutz](PRIVACY.md) ·
[Manager entfernen](UNINSTALL.md) · [SignPath einrichten](SIGNPATH_SETUP.md).

Der offene Programmcode ändert die Registrierung und Freischaltung der ModBase
nicht. Der Server entscheidet weiterhin über jeden geschützten Abruf.

## Erst das Forum aktualisieren

Im WoltLab-ACP **de.siri-mods.modbase_2.0.0_Beta_2.tar.gz** als Paketupdate
installieren. Unterstützt: ModBase 2.0.0 Beta 1 und 1.0.0 Beta 10; auch
Neuinstallation. Die API muss in den ModBase-Optionen aktiviert sein.
Die Anmeldung funktioniert mit diesem Pluginupdate. Der ältere Manager
0.7.2 unterstützt den jetzt geschlossenen Zugang nicht.

## Installieren und anmelden

1. Alten Manager schließen und **SiriModManager_Setup_v0.8.0.exe** ausführen.
   Installation unter `%LOCALAPPDATA%\Programs\Siri ModManager`, mit
   Desktop-Icon und Startmenü-Eintrag. Mods und Spielpfade bleiben erhalten.
2. Im Manager **Im Forum anmelden** anklicken.
3. Im normalen Browser mit dem vorhandenen Forumkonto anmelden. Die
   Registrierung allein reicht nicht: Das Konto benötigt eure Freischaltung.
4. Auf der Forumseite **Diesen PC verbinden** bestätigen. Danach zum Manager
   zurückwechseln. Er lädt den Katalog und prüft installierte Versionen.

Die Portable-ZIP wird entpackt; anschließend `SiriModManager.exe` starten.
Sie speichert Einstellungen ebenfalls im Windows-Benutzerprofil.

Unter **Forumkonto** oder **Einstellungen → Forumkonto** gibt es Anmeldung,
Abmeldung, Registrierung, Freischaltung und Geräteverwaltung. Im Forum ist
**Verbundene Geräte** auch in der ModBase-Navigation erreichbar.
Die Browseranmeldung kann im Manager abgebrochen werden und läuft nach
fünf Minuten ab. Das Forum-Passwort wird im Manager weder eingegeben noch
angefordert. Eine erforderliche Zwei-Faktor-Anmeldung bleibt verpflichtend.

## Zugang und Geräte

- Es gelten aktiviertes, ungesperrtes Forumkonto, eure ModBase-Freischaltung,
  Benutzergruppen und die Lese-/Downloadrechte der jeweiligen Veröffentlichung.
- Einträge, Bilder und Downloads werden bei jedem Abruf auf dem Server geprüft.
- Unter **Geräte verwalten** lassen sich einzelne PCs jederzeit abmelden.
- Bei Entzug der Freigabe sind weitere Abrufe und Downloads gesperrt. Eine
  bereits laufende Übertragung kann beendet werden. Installierte Mods bleiben.
- Zugriffstokens gelten zehn Minuten. Die Erneuerung erfolgt automatisch mit
  rotierenden Tokens, solange die Freigabe besteht. Nach 30 Tagen ohne
  Erneuerung oder spätestens nach 90 Tagen neu im Browser anmelden.
- Tokens werden unter Windows mit DPAPI an das Windows-Benutzerkonto gebunden
  verschlüsselt gespeichert. Der Server speichert ausschließlich Token-Hashes.
- Kein gemeinsamer API-Schlüssel in der EXE. Keine Foren-Cookies im Manager.
- Bei einer unterbrochenen Token-Erneuerung kann eine erneute Anmeldung nötig
  werden: Bereits verbrauchte Erneuerungstokens werden nicht erneut verwendet.

## Mods, Details und Updates

Suchfeld, Spielauswahl und Kategorien bleiben erhalten. Dunkel ist Standard;
der Hellmodus kann unter Einstellungen eingeschaltet werden. Details oder
Modname öffnen ein eigenes Fenster mit X, Schließen und Esc. Es zeigt
Beschreibung, Funktionen, Menüoptionen, Installation, technische Angaben,
Lizenz, Versionsverlauf, verfügbare Dateien, Zusatzfelder und Vorschaubild.

Neue freigegebene Veröffentlichungen erscheinen bei der nächsten Abfrage.
Ankündigungen können als veröffentlichte Einträge ohne Release angezeigt
werden. Private Entwürfe und zurückgezogene Releases gehören nicht zum
Desktop-Katalog. Bei mehreren Versionen wird die neueste stabile Version
bevorzugt; wenn keine stabile existiert, die neueste sichtbare Version.

Update-Prüfung beim Start und alle 15 Minuten, solange der Manager geöffnet
ist. Die periodische Prüfung lässt sich abschalten; nach Start/Anmeldung wird
der Zugang einmal geprüft. Updates werden angezeigt und per Klick installiert.

Direkt installiert werden ZIP-Dateien. Ein ZIP kann mehrere Modordner enthalten.
Hat ein Release mehrere ZIP-Varianten, öffnet der Manager zur Auswahl die
ModBase, statt eine Variante zu erraten. RAR/7z und reine Workshop-Links werden
nicht direkt installiert. Voraussetzung bleibt ein geeignetes Modpaket.

Der Download wird zunächst vollständig geladen, auf Größe/Prüfsumme geprüft
und entpackt. Danach ersetzen die neuen Ordner die bisherigen vollständig.
Die temporäre Sicherung wird nach Erfolg gelöscht; bei Fehlern wird der alte
Stand wiederhergestellt. Nach einem Prozessabbruch erfolgt die Wiederherstellung
beim nächsten Einlesen. Alte und neue Dateien werden nicht vermischt.

Deinstallieren zeigt die betroffenen Modordner und fragt nach. Vor Updates
und Deinstallation das Spiel schließen. Eigene Änderungen in ersetzten
Modordnern gehen verloren. Spielstände bleiben unverändert.

## TF2 und TF3 getrennt

Unter Einstellungen für jedes Spiel **Ordner auswählen …** anklicken.
Der Windows-Ordnerdialog öffnet sich. Der neue Pfad wird gespeichert und
neu eingelesen; vorhandene Mods werden bei einem Pfadwechsel nicht verschoben.
TF2 und TF3 müssen verschiedene Modordner verwenden.

Der Manager sucht vorhandene persönliche TF2-Steam-Modordner unter
`Steam\userdata\<Steam-ID>\1066780\local\mods`. Bei mehreren Konten selbst
wählen. GOG-/Epic-Pfade sind manuell auswählbar. TF3 erhält einen separat
gewählten Modordner. Kompatibilität mit konkreten TF3-Paketen muss im Spiel
geprüft werden; die getrennte Dateiverwaltung allein bestätigt sie nicht.

Installationen dieses Managers tragen `.siri-modmanager.json` in ihren
Modordnern. Das ermöglicht verlässliche Zuordnung von Eintrag und Version.
Manuell installierte Mods ohne nachweisbare Zuordnung/Version bleiben lokal
sichtbar und werden nicht anhand der neuesten API-Version als aktuell erraten.
Steam-Workshop-Abonnements werden von Steam verwaltet.

## Lokale Daten

`%APPDATA%\SiriModManager` enthält `config.json`, `installed.json`,
`account.dpapi`, Icon und Fehlerprotokoll. Keine Tokens in Konfiguration oder
Protokoll. Der alte öffentliche `mods_cache.json` wird gelöscht. Geschützte
Katalogdaten bleiben nur im Arbeitsspeicher und werden bei Abmeldung entfernt.
Installierte Mods können auch ohne Anmeldung lokal verwaltet werden.

Die API-Adresse steht in `config.json` als `api_url`. Standard:
https://forum.siri-mods.de/index.php?siri-modbase-api/

Anmeldeadressen werden von dieser API ermittelt. API, Anmelde- und Datei-URLs
müssen HTTPS und denselben Server verwenden. Ein Wechsel der API erfordert
neue Anmeldung. Bei einer geänderten Forumroute die neue API-Adresse aus dem
ACP übernehmen. Weiterleitung auf einen fremden Server ist gesperrt.

## Bauen und testen

Go 1.24 oder neuer, keine externen Go-Abhängigkeiten; der vorbereitete GitHub-Build
verwendet Go 1.27.1. Für lokale Windows-Builds außerdem PowerShell und MinGW-w64
mit `windres` und GCC installieren. Beispiel bei MSYS2 unter `C:\msys64`:

```powershell
./Build.ps1 -WindResPath C:\msys64\mingw64\bin\windres.exe
```

Die Ausgabe unter `dist/unsigned` ist unsigniert. Icon, Manifest und
Versionsressourcen werden bei jedem Build aus `.rc` und `.ico` neu erzeugt;
vorkompilierte `.syso`- oder EXE-Dateien gehören nicht ins Repository.
Das Setup bettet die zuvor gebaute Manager-EXE ein. Der SignPath-Workflow
verwendet dafür ausdrücklich die bereits signierte und geprüfte Manager-EXE.

`Build and test` kann ohne SignPath-Konto laufen. `SignPath signed build` wird erst
nach der Einrichtung in [SIGNPATH_SETUP.md](SIGNPATH_SETUP.md) manuell gestartet.
Die Anleitung enthält auch die erforderlichen Projekt-, Zertifikat- und Tokenwerte.

Kernlogik: `go test -race -count=1 ./core` (benötigt lokale Testserverports).
Auf Windows zusätzlich: `go test ./internal/win` für den realen DPAPI-Test.

Geprüft: PKCE/Callback-Bindung, Abbruch, parallele Token-Erneuerung, Widerruf,
Herkunftsbindung, Schreibschutz, sicherer Speicherfehler, Plugin-v3-JSON sowie
bestehende Installations-, Update-, Deinstallations-, Pfad- und Detailtests.
Der Quellcode enthält reproduzierbare Tests und den API-Vertrag als Prüfdaten.

Prüfgrenzen: Windows-Oberfläche, Explorer, DPAPI und Setup wurden für Windows
x64 kompiliert, hier jedoch nicht auf einem Windows-PC ausgeführt. Das Plugin
wurde nicht auf eurem Live-Forum installiert. Ein vollständiger Browserlogin
gegen eure produktive WoltLab-Installation ist nach dem Paketupdate zu prüfen.
