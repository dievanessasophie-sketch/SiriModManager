# Datenschutz im Siri ModManager

Stand: 3. Oktober 2026. Gilt für die Desktop-Anwendung Version 0.8.0.
Projektkontakt: Siri, https://www.siri-mods.de/, mail@siri-mods.de.

## Netzwerkverbindungen

Der Manager verbindet sich mit der in den Einstellungen gespeicherten ModBase-API.
Standard ist https://forum.siri-mods.de/index.php?siri-modbase-api/.
Für die Anmeldung öffnet er den normalen Browser. Das Passwort wird auf der
Forumseite eingegeben. Die Anwendung erhält einen Zugriff für dieses Gerät und
speichert ein Erneuerungstoken verschlüsselt mit Windows DPAPI.

Nach erfolgreicher Anmeldung fragt der Manager beim Start und, falls aktiviert,
alle 15 Minuten die Modliste und aktuelle Versionen ab. Beim Öffnen von Details
kann ein Vorschaubild geladen werden. Moddateien werden nach Auswahl einer
Installation oder eines Updates heruntergeladen. Authentifizierte Abrufe werden
auf dieselbe HTTPS-Herkunft beschränkt. Links zur ModBase oder anderen Angeboten
werden nach einem Klick im Browser geöffnet.

Der angesprochene Server erhält technisch notwendige Verbindungsdaten, etwa
IP-Adresse, User-Agent mit Manager-Version, angeforderte Ressourcen und das
Zugriffstoken. Bei der Geräteanmeldung wird der konfigurierte Gerätename
übermittelt. Verarbeitung und Aufbewahrung auf dem Forum richten sich nach der
dort veröffentlichten Datenschutzerklärung. Für im Browser geöffnete andere
Angebote gelten deren Datenschutzhinweise.

Der Manager enthält keine Werbe-, Analyse- oder Tracking-SDKs und lädt keine
Spielstände, lokalen Modordner oder Fehlerprotokolle automatisch hoch.
SignPath gehört zum Release-Build; der installierte ModManager verwendet keine
SignPath-API. Windows kann Zertifikate über eigene Sicherheitsdienste prüfen.

## Lokale Daten und Kontrolle

Unter `%APPDATA%\SiriModManager` liegen Konfiguration, gewählte Spielpfade,
Installationszuordnungen, verschlüsselte Gerätetokens, Icon und ein lokales
Fehlerprotokoll. Das Protokoll kann Dateipfade und technische Fehlermeldungen
enthalten; vor einem freiwilligen Support-Upload selbst prüfen.

Geschützte Katalogdaten bleiben im Arbeitsspeicher. In installierten Modordnern
speichert `.siri-modmanager.json` die Zuordnung von Mod und Version.

Automatische regelmäßige Abfragen lassen sich unter Einstellungen abschalten.
Abmelden entfernt die lokalen Tokens und versucht, den Gerätezugriff auf dem
Server zu widerrufen. Bei fehlender Verbindung kann der Serverwiderruf scheitern;
in diesem Fall das Gerät zusätzlich im Forum unter „Verbundene Geräte“ entfernen.
Die lokale Verwaltung installierter Mods ist ohne Anmeldung möglich.

Die vollständige Entfernung des Programms und optional seiner lokalen Daten ist
in [UNINSTALL.md](UNINSTALL.md) beschrieben.
