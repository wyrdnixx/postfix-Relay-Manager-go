# Postfix Relay Manager

Webbasiertes Verwaltungswerkzeug für einen Postfix-Server, der als SMTP-Relay für interne Geräte (Drucker, Scanner, Server) fungiert. Über die Weboberfläche werden Client-Systeme registriert, Relay-Server konfiguriert und Postfix-Einstellungen automatisch angewendet.

---

## Funktionsumfang

| Seite | Funktion |
|---|---|
| **Startseite** | Übersicht aller registrierten Systeme, hinzufügen / bearbeiten / löschen |
| **Bulk-Import** | Mehrere IP-Adressen auf einmal importieren (Zeilenumbruch- oder Komma-getrennt) |
| **Postfix-Status** | Warteschlange, Dienststatus, Aktionen (Resend, Löschen, Neustart), Test-Mail |
| **Protokoll** | Tab 1: abgelehnte Relay-Versuche unbekannter IPs — Tab 2: Zustellprotokoll mit Absender-IP, Queue-ID, Relay, Status |
| **Systemprüfung** | Automatische Diagnose der Postfix-Konfiguration und Relay-Erreichbarkeit |
| **Einstellungen** | Relay-Server konfigurieren, Passwort ändern |

Der **Queue-Badge** in der Navigation zeigt die aktuelle Anzahl wartender Mails und verlinkt direkt zur Postfix-Statusseite.

---

## Voraussetzungen

- Linux-Server mit installiertem und laufendem Postfix
- Go 1.22 oder neuer (nur zum Selbstkompilieren, nicht nötig wenn das enthaltene Binary verwendet wird)
- Root-Berechtigung oder `sudo` (für Postfix-Konfiguration und `/etc/hosts`)

---

## Installation

### Option A – Vorcompiliertes Binary (empfohlen)

Das Repository enthält ein fertig kompiliertes Binary für Linux/amd64:

```bash
git clone <repository-url> postfix-relay-manager
cd postfix-relay-manager
sudo mkdir -p /opt/postfix-relay-manager
sudo cp postfix-relay-manager /opt/postfix-relay-manager/
sudo cp data.json.example /opt/postfix-relay-manager/data.json
```

### Option B – Selbst kompilieren

```bash
git clone <repository-url> postfix-relay-manager
cd postfix-relay-manager
go build -o postfix-relay-manager .
sudo mkdir -p /opt/postfix-relay-manager
sudo cp postfix-relay-manager /opt/postfix-relay-manager/
sudo cp data.json.example /opt/postfix-relay-manager/data.json
```

---

### `data.json` konfigurieren

```bash
sudo nano /opt/postfix-relay-manager/data.json
```

Mindestens diese Felder unter `config` müssen gesetzt sein:

| Feld | Beschreibung |
|---|---|
| `adminUsername` | Login-Benutzername |
| `adminPasswordHash` | SHA-256-Hash des Passworts (siehe unten) |
| `relayServersInternal` | Upstream-Relay-Server für interne Systeme |
| `relayServersExternal` | Upstream-Relay-Server für externe Systeme |

**Passwort-Hash erzeugen:**
```bash
echo -n 'MEIN_PASSWORT' | sha256sum | cut -d' ' -f1
```

### Systemd-Service einrichten

```bash
sudo cp postfix-relay-manager.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now postfix-relay-manager
```

### Status prüfen

```bash
sudo systemctl status postfix-relay-manager
```

---

## Konfiguration der Relay-Server

Die Relay-Server werden über **Einstellungen → Relay-Server** konfiguriert und lassen sich dort jederzeit ändern. Nach dem Klick auf **Speichern & Anwenden** werden alle Postfix-Konfigurationsdateien sofort aktualisiert und Postfix neu geladen.

### Intern / Extern

- **Intern**: Upstream-Server für interne Systeme (z. B. Drucker, Scanner im LAN)
- **Extern**: Upstream-Server für externe Systeme (z. B. DMZ-Server mit Internet-Relay)

Alle Server einer Gruppe verwenden **denselben Port**. Es können beliebig viele Server pro Gruppe eingetragen werden.

### Automatischer Failover

Der Postfix Relay Manager konfiguriert Postfix so, dass bei Ausfall eines Relay-Servers automatisch der nächste versucht wird:

1. Alle Server einer Gruppe werden als A-Records eines virtuellen Hostnamens in `/etc/hosts` eingetragen:
   ```
   # --- postfix-relay-manager begin ---
   192.168.1.10    relay-int.prm
   192.168.1.11    relay-int.prm
   10.0.0.10       relay-ext.prm
   10.0.0.11       relay-ext.prm
   # --- postfix-relay-manager end ---
   ```

2. Postfix verwendet diesen Hostnamen als `relayhost`:
   ```
   relayhost = [relay-int.prm]:26
   ```

3. Schlägt die Verbindung zum ersten Server fehl, versucht Postfix automatisch den nächsten A-Record — ohne Verzögerung und ohne manuelle Eingriffe.

> **Wichtig:** Alle Server einer Gruppe müssen über denselben Port erreichbar sein, da Postfix den Port aus dem `relayhost`-Eintrag bezieht, nicht aus den einzelnen A-Records.

Da Postfix seinen SMTP-Client in einem Chroot unter `/var/spool/postfix/` betreibt, schreibt der Manager den verwalteten Block zusätzlich nach `/var/spool/postfix/etc/hosts`.

---

## Von Postfix verwaltete Konfigurationsdateien

Der Manager schreibt und aktualisiert folgende Dateien automatisch bei jeder Konfigurationsänderung:

| Datei | Inhalt |
|---|---|
| `/etc/postfix/allowed_clients` | Zuordnung Client-IP → Relay-Transport (FILTER) |
| `/etc/postfix/main.cf` | `mynetworks`, `relayhost`, `inet_interfaces`, `inet_protocols`, `smtp_host_lookup` |
| `/etc/hosts` | Failover-A-Records für `relay-int.prm` / `relay-ext.prm` |
| `/var/spool/postfix/etc/hosts` | Kopie des obigen Blocks für den Postfix-Chroot |

Nach jeder Änderung werden `postmap` und `systemctl reload postfix` automatisch ausgeführt.

### Einmalige manuelle Anpassung in `main.cf`

Diese Zeile muss einmalig manuell in `/etc/postfix/main.cf` eingetragen werden (falls noch nicht vorhanden):

```
smtpd_relay_restrictions =
    permit_mynetworks
    check_client_access hash:/etc/postfix/allowed_clients
    reject
```

Die Systemprüfung weist darauf hin, falls der Verweis auf `allowed_clients` fehlt.

---

## Beispiel `data.json`

```json
{
  "systems": [],
  "baseMynetworks": "",
  "allManagedIps": [],
  "config": {
    "relayServersInternal": [
      { "host": "192.168.1.10", "port": 26 },
      { "host": "192.168.1.11", "port": 26 }
    ],
    "relayServersExternal": [
      { "host": "10.0.0.10", "port": 27 },
      { "host": "10.0.0.11", "port": 27 }
    ],
    "allowedClientsFile": "/etc/postfix/allowed_clients",
    "mainCfFile": "/etc/postfix/main.cf",
    "mailLogFile": "/var/log/mail.log",
    "port": "8080",
    "adminUsername": "admin",
    "adminPasswordHash": "057ba03d6c44104863dc7361fe4578965d1887360f90a0895882e58a6248fc86"
  }
}
```

> Der Hash entspricht dem Passwort `changeme` — **vor dem ersten produktiven Einsatz ersetzen.**

---

## Optionale Konfigurationsfelder

| Feld | Standard | Beschreibung |
|---|---|---|
| `config.port` | `8080` | HTTP-Port der Weboberfläche |
| `config.allowedClientsFile` | `/etc/postfix/allowed_clients` | Pfad zur allowed_clients-Datei |
| `config.mainCfFile` | `/etc/postfix/main.cf` | Pfad zur main.cf |
| `config.mailLogFile` | `/var/log/mail.log` | Pfad zum Mail-Log (Fallback: journalctl) |

---

## Passwort ändern

**Option A – Weboberfläche:** Einstellungen → Passwort ändern.

**Option B – manuell:**
```bash
echo -n 'NEUES_PASSWORT' | sha256sum | cut -d' ' -f1
# Hash in data.json unter config.adminPasswordHash eintragen
sudo systemctl restart postfix-relay-manager
```

---

## Systemprüfung

Die Seite **Systemprüfung** prüft automatisch:

- Postfix installiert und Dienst aktiv
- `postmap` verfügbar
- `main.cf` lesbar und beschreibbar
- `mynetworks` in `main.cf` vorhanden
- `allowed_clients` in `main.cf` referenziert
- sudo-Berechtigung für `postmap` / `systemctl reload`
- `relayhost` konfiguriert (mit Hinweis auf aktiven Failover)
- `smtp_host_lookup = native` gesetzt (erforderlich damit `/etc/hosts` aufgelöst wird)
- `inet_interfaces` nicht auf `loopback-only` beschränkt
- TCP-Erreichbarkeit aller konfigurierten Relay-Server

---

## Deinstallation

```bash
sudo systemctl stop postfix-relay-manager
sudo systemctl disable postfix-relay-manager
sudo rm /etc/systemd/system/postfix-relay-manager.service
sudo systemctl daemon-reload
sudo rm -rf /opt/postfix-relay-manager

# Verwalteten /etc/hosts-Block entfernen
sudo sed -i '/# --- postfix-relay-manager begin ---/,/# --- postfix-relay-manager end ---/d' /etc/hosts
sudo sed -i '/# --- postfix-relay-manager begin ---/,/# --- postfix-relay-manager end ---/d' /var/spool/postfix/etc/hosts

# Postfix-Konfiguration bereinigen
sudo postconf -e 'relayhost='
sudo systemctl reload postfix
```
