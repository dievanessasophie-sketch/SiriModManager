# SignPath Foundation einrichten

Stand: 3. Oktober 2026. Dies ist ein vorbereitetes Projekt, keine Zusage der
Foundation. Die vorhandenen EXEs sind noch unsigniert. Für die Aufnahme verlangt
SignPath ein aktives, öffentliches Open-Source-Projekt, eine passende Lizenz,
veröffentlichte Software und nachvollziehbare Builds. Eine Aufnahme ist nicht
garantiert, insbesondere bei einem neuen Projekt mit noch geringer Bekanntheit.

## 1. Öffentliches Repository

Das öffentliche Repository ist angelegt und vollständig eingerichtet:
https://github.com/dievanessasophie-sketch/SiriModManager

Quellcode, `.github`, `.gitignore` und `.gitattributes` liegen in der Wurzel.
Hauptbranch: `main`. Die Freigabe des MIT-Quellcodes wurde bestätigt.

Die vorbereitete Lizenz ist **MIT**. Sie erlaubt ausdrücklich auch Änderungen,
Weitergabe und kommerzielle Nutzung des Manager-Codes. Diese Lizenzentscheidung
vor Veröffentlichung prüfen. Die Mods und der Forum-Servercode sind nicht in
diesem Quellcodepaket enthalten; ihre eigenen Rechte und Zugangsvoraussetzungen
werden dadurch nicht geändert. Im Paket sind keine Zugangstokens enthalten.

Siri als tatsächliche Maintainerin hinterlegen und GitHub-MFA aktivieren.
Das Repository selbst kann als öffentlich erreichbare Projekt-Homepage dienen.
README, Datenschutz, Deinstallationsanleitung und Code signing policy müssen
ohne Forum-Anmeldung erreichbar sein.

## 2. Build und erste Veröffentlichung

Der erste Windows-Build war am 3. Oktober 2026 erfolgreich:
https://github.com/dievanessasophie-sketch/SiriModManager/actions/runs/37091483292

Unter **Actions → Build and test** lassen sich weitere Builds starten. Der Workflow prüft die
Kernfunktionen und Windows-DPAPI, erzeugt Versionsressourcen aus dem Quellcode
und baut Manager und Setup auf einem von GitHub bereitgestellten Windows-Rechner.
Es werden keine mitgelieferten EXEs oder `.syso`-Dateien übernommen.

Das Ergebnis heißt **SiriModManager-UNSIGNED-...**. Vor Veröffentlichung auf einem
Windows-PC prüfen: Start, Setup, Desktop-Verknüpfung, Forumlogin mit freigegebenem
Konto, Ablehnung ohne Freigabe, Abmelden und lokale Modverwaltung.

Der öffentliche Release zur geprüften Quellcodeversion wurde veröffentlicht:
https://github.com/dievanessasophie-sketch/SiriModManager/releases/tag/v0.8.0

Er ist deutlich als unsigniert gekennzeichnet. EXEs, ZIP, Lizenzhinweise und
Prüfsummen sind verfügbar; die Beschreibung verlinkt Anleitung, Datenschutz,
Deinstallation und Signierstatus.
Die GitHub-Actions-Artefakte allein ersetzen keinen dauerhaften öffentlichen Release.

## 3. Antrag bei der Foundation

https://signpath.org/apply öffnen. Projektname, Repository, Homepage,
öffentlicher Release und Maintainerkonto angeben. Eine englische Textvorlage
liegt vollständig mit den echten Projekt- und Release-Links als
`signpath/ANTRAG.txt` bei. Die Vorlage behauptet keine bestehende Freigabe oder laufende
Produktivsignierung. Der Antrag ist noch nicht versendet.

Die Foundation prüft das Projekt und kann weitere Voraussetzungen verlangen.
Bei Annahme erstellt sie den Zugang bzw. erläutert die weiteren Einrichtungsschritte.
Das Herausgeberzertifikat gehört zur **SignPath Foundation**. Der angezeigte
Windows-Herausgeber ist daher nicht frei als „siri-mods.de“ wählbar.

## 4. SignPath und GitHub verknüpfen

Diese Schritte erst nach der Aufnahme ausführen, mit den tatsächlich zugeteilten Werten:

1. Projekt in SignPath anlegen/übernehmen und an genau dieses GitHub-Repository
   mit **GitHub.com als Trusted Build System** binden. Die offizielle SignPath
   GitHub App nur für das betreffende Repository berechtigen.
2. Zwei Artifact Configurations unter den Slugs `manager` und `setup` erstellen.
   `signpath/manager.xml` und `signpath/setup.xml` importieren. Beide verlangen
   einen Versionsparameter und prüfen Produktname, Firma, Originaldateiname
   und Versionsnummer. Das ZIP-Wurzelelement ist für das GitHub-Artefakt vorgesehen.
3. Je eine Test- und Produktionsrichtlinie konfigurieren. Der Produktionsrichtlinie
   das echte Foundation-Zertifikat und einen Zeitstempeldienst zuordnen. Bei beiden
   einen tatsächlichen Approver hinterlegen; Freigabe pro Signaturanfrage verlangen.
   Den API-Nutzer nur als Submitter berechtigen. MFA auf allen beteiligten Konten.
4. In SignPath Herkunftsprüfung, GitHub-hosted Runner und die zugelassenen
   Repository-/Branch-/Tag-Regeln aktivieren bzw. von der Foundation bestätigen
   lassen. Die Prüfungen im Workflow ersetzen diese serverseitigen Regeln nicht.
5. GitHub: **Settings → Environments → New environment → `signpath`**.
   Einen berechtigten Reviewer und Regeln für `main` sowie freigegebene `v*`-Tags
   konfigurieren. Mainbranch und Release-Tags vor unkontrollierten Änderungen
   schützen. Die veröffentlichten Versionstags nicht auf andere Commits verschieben.
6. Die folgenden Werte als Variablen/Secret im Environment `signpath` hinterlegen.

| Art | Name | Wert |
| --- | --- | --- |
| Secret | `SIGNPATH_API_TOKEN` | Token des SignPath-Submitters |
| Variable | `SIGNPATH_ORGANIZATION_ID` | Zugewiesene SignPath-Organisations-ID |
| Variable | `SIGNPATH_PROJECT_SLUG` | Tatsächlicher Slug des Manager-Projekts |
| Variable | `SIGNPATH_TEST_POLICY` | Slug der Test-Richtlinie |
| Variable | `SIGNPATH_RELEASE_POLICY` | Slug der Produktionsrichtlinie |
| Variable | `SIGNPATH_TEST_SUBJECT` | Exakter Subject-String des Testzertifikats |
| Variable | `SIGNPATH_RELEASE_SUBJECT` | Exakter Subject-String des Foundation-Zertifikats |

Die Subject-Werte stammen aus den öffentlich einsehbaren Zertifikaten, nicht aus
einem frei gewählten Publishernamen. Unter Windows lässt sich der Wert einer von
SignPath erhaltenen öffentlichen `.cer`-Datei mit `(Get-PfxCertificate .\cert.cer).Subject`
anzeigen. Keine privaten Schlüssel oder API-Tokens im Repository veröffentlichen.

## 5. Test und Produktionssignierung

**Actions → SignPath signed build → Run workflow → main / test** starten.
Der Approver bestätigt nacheinander die Manager- und Setup-Anfrage in SignPath.
Pro Anfrage wartet der Workflow bis zu 40 Minuten. Testdateien tragen `_TEST`;
sie werden nicht als öffentlich vertrauenswürdige Releases ausgegeben.
Es wird kein Testzertifikat automatisch auf Rechnern als vertrauenswürdig installiert.

Für den echten Release muss der Tag **v0.8.0** auf den geprüften Commit zeigen
(bei späteren Versionen entsprechend ändern). Workflow für diesen Tag mit
**release** starten und beide Signaturanfragen bestätigen. Er prüft Herausgeber,
Signaturstatus, Produktversion und Zeitstempel. Der bereits signierte Manager
wird unverändert in das Setup eingebettet. Danach werden Setup, Manager,
Portable-ZIP, Lizenzhinweise und SHA256SUMS.txt bereitgestellt.

Die fertigen Dateien nach Sichtprüfung veröffentlichen. Der Workflow lädt sie
als Build-Artefakt hoch; er veröffentlicht keinen Release automatisch.
In README und CODE_SIGNING.md erst jetzt den bestätigten Status eintragen und
den dort vorbereiteten SignPath-Hinweis auf den Downloadseiten ergänzen.

## Grenzen und Aktualisierung

Eine Signatur beseitigt nicht garantiert sofort jede SmartScreen-Meldung.
SignPath entscheidet über Aufnahme, Zertifikat und zusätzliche Kontrollen.
Die GitHub-Ausführung einschließlich Windows-DPAPI-Tests wurde erfolgreich
geprüft. Echte Signatur, manuelle Signierfreigaben und die Windows-Oberfläche
sind weiterhin offen. Details stehen in [PRUEFBERICHT.txt](PRUEFBERICHT.txt). Der Workflow bricht bei fehlenden Werten oder ungültiger
Signatur ab und liefert keinen ersatzweise unsignierten „Release“.

GitHub-Actions sind auf geprüfte Commit-IDs festgelegt (checkout v6, setup-go v6,
upload-artifact v7, MSYS2 v2, SignPath v3). Aktualisierungen bewusst prüfen.
Die MSYS2-Pakete werden bei jedem Build aus dessen offiziellen Repositories
installiert; es wird keine bitgenaue Reproduzierbarkeit über beliebige Zeiträume
behauptet. SignPath prüft die Herkunft des jeweiligen GitHub-Builds.

Quellen: https://signpath.org/terms · https://signpath.org/apply ·
https://docs.signpath.io/trusted-build-systems/github ·
https://docs.signpath.io/artifact-configuration/
