# 0.8.1: ZIPs ohne übergeordneten Modordner

Die authentifizierte ModBase-API liefert den Originaldateinamen, bisher jedoch kein `folder`. Bei genau einem ZIP in der ausgewählten Veröffentlichung übernimmt der Manager jetzt dessen Namen ohne `.zip` als Zielordner, sofern er eine sichere Kennung nach `autor_name_nummer` ist. Beispiel: `siri_dortmund_1.zip` wird direkt nach `mods/siri_dortmund_1/` entpackt. Groß-/Kleinschreibung der Kennung bleibt erhalten. Explizites `folder` am Eintrag, danach an der Datei, hat Vorrang. Ungültige explizite Angaben werden nicht stillschweigend ersetzt.

ZIPs mit eigenem Modordner verwenden weiterhin diesen. Bei TF3 wird ein Paket mit `metadata/mod.lua` als Ganzes erkannt, damit auch sein Inhalt installiert wird. Beliebige Downloadtitel, Pfade oder Versionszusätze werden nicht in Modkennungen umgeschrieben: Dafür ist ein explizites `folder` erforderlich. Die bestehende API muss für kanonisch benannte ZIPs nicht geändert werden.

Regressionstests prüfen beide Spiele mit flachen ZIPs über die API-Auswertung, Installation, Update (Entfernung alter Dateien), Deinstallation, Schutz anderer Mods und unveränderte Ordnerstruktur. Die bestehende Testsuite läuft ebenfalls. Ein manueller Test mit dem betroffenen produktiven Archiv und im Spiel steht noch aus.

Build 0.8.1 bleibt unsigniert. 0.8.0 wird durch diese Quellcodeänderung nicht nachträglich verändert.
