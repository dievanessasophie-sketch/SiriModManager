# Siri ModManager entfernen

Diese Schritte entfernen den Manager. Mods können vorher im Manager einzeln
über „Deinstallieren“ entfernt werden; ohne diese Aktion bleiben sie erhalten.

1. Im Manager unter **Forumkonto → Abmelden** abmelden. Falls keine Verbindung
   besteht, den Gerätezugriff später im Forum unter **Verbundene Geräte** widerrufen.
2. Den Manager vollständig schließen.
3. Im Explorer `%LOCALAPPDATA%\Programs\Siri ModManager` öffnen und den
   Programmordner entfernen. Bei der Portable-Version deren entpackten Ordner entfernen.
4. Die Verknüpfung **Siri ModManager** auf dem Desktop und im Startmenü entfernen.
   Den Startmenü-Eintrag findest du über Windows+R → `shell:programs`.
5. Optional `%APPDATA%\SiriModManager` entfernen, wenn auch Einstellungen,
   lokale Installationsübersicht und Fehlerprotokoll gelöscht werden sollen.

Spielordner, Spielstände und Moddateien werden durch diese Schritte nicht gelöscht.
Der Manager installiert keinen Dienst und keinen Autostart-Eintrag. Das bisherige
Setup besitzt keinen eigenen Eintrag unter „Installierte Apps“; deshalb erfolgt
die Entfernung wie oben beschrieben.
