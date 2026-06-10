# Postfix Relay Manager

Webbasiertes Verwaltungswerkzeug für Postfix-Relay-Berechtigungen. Erlaubt das Hinzufügen, Bearbeiten und Löschen von Client-Systemen (Drucker, Server, etc.), die über den Postfix-Server E-Mails senden dürfen. Änderungen werden automatisch in `/etc/postfix/allowed_clients` und `main.cf` geschrieben und Postfix wird neu geladen.

## Voraussetzungen

- Linux-Server mit laufendem Postfix
- Go 1.22 oder neuer (nur zum Kompilieren)
- `sudo`-Berechtigung oder Root-Ausführung (für `postmap` und `systemctl reload postfix`)

---

## Installation

### 1. Quellcode kompilieren

```bash
git clone <repository-url> postfix-relay-manager
cd postfix-relay-manager
go build -o postfix-relay-manager .
```

### 2. Verzeichnis anlegen und Dateien kopieren

```bash
sudo mkdir -p /opt/postfix-relay-manager
sudo cp postfix-relay-manager /opt/postfix-relay-manager/
```

### 3. `data.json` anlegen

```bash
sudo cp data.json.example /opt/postfix-relay-manager/data.json
sudo nano /opt/postfix-relay-manager/data.json
```

Die `data.json` muss mindestens die Felder unter `config` enthalten (siehe Abschnitt [Konfiguration](#konfiguration)).

### 4. Passwort-Hash generieren

Das Passwort wird als SHA-256-Hash in `data.json` gespeichert. Hash erzeugen:

```bash
echo -n 'MEIN_PASSWORT' | sha256sum | cut -d' ' -f1
```

Den ausgegebenen Hash in `data.json` unter `config.adminPasswordHash` eintragen.

### 5. Systemd-Service einrichten

```bash
sudo cp postfix-relay-manager.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable postfix-relay-manager
sudo systemctl start postfix-relay-manager
```

### 6. Status prüfen

```bash
sudo systemctl status postfix-relay-manager
sudo journalctl -u postfix-relay-manager -f
```

---

## Konfiguration

Die gesamte Konfiguration befindet sich in `/opt/postfix-relay-manager/data.json` neben der ausführbaren Datei.

### Pflichtfelder

| Feld | Beschreibung |
|---|---|
| `config.adminUsername` | Login-Benutzername |
| `config.adminPasswordHash` | SHA-256-Hash des Passworts |
| `config.relayServersInternal` | Relay-Server für interne Systeme |
| `config.relayServersExternal` | Relay-Server für externe Systeme |

Fehlen diese Felder, beendet sich das Programm beim Start mit einer Fehlermeldung.

### Optionale Felder

| Feld | Standard | Beschreibung |
|---|---|---|
| `config.port` | `8080` | HTTP-Port der Weboberfläche |
| `config.allowedClientsFile` | `/etc/postfix/allowed_clients` | Pfad zur allowed_clients-Datei |
| `config.mainCfFile` | `/etc/postfix/main.cf` | Pfad zur main.cf |
| `config.mailLogFile` | `/var/log/mail.log` | Pfad zum Mail-Log (Fallback: journalctl) |

---

## Beispiel `data.json`

```json
{
  "systems": [],
  "baseMynetworks": "",
  "allManagedIps": [],
  "config": {
    "relayServersInternal": [
      {
        "host": "10.100.0.35",
        "port": 26
      },
      {
        "host": "10.100.0.36",
        "port": 26
      }
    ],
    "relayServersExternal": [
      {
        "host": "10.100.0.31",
        "port": 27
      },
      {
        "host": "10.100.0.32",
        "port": 27
      }
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

> Der oben eingetragene Hash entspricht dem Passwort `changeme`.  
> **Vor dem ersten produktiven Einsatz unbedingt durch einen eigenen Hash ersetzen.**

---

## Passwort ändern

### Option A – Weboberfläche

Nach dem Login unter **Einstellungen** → **Passwort ändern**.  
Der neue Hash wird automatisch in `data.json` gespeichert.

### Option B – Manuell in `data.json`

```bash
# Neuen Hash erzeugen
echo -n 'NEUES_PASSWORT' | sha256sum | cut -d' ' -f1

# data.json bearbeiten
sudo nano /opt/postfix-relay-manager/data.json

# Service neu starten
sudo systemctl restart postfix-relay-manager
```

---

## Postfix-Konfiguration

Der Manager schreibt zwei Dateien:

**`/etc/postfix/allowed_clients`** – Weist jede Client-IP einem Relay-Server zu:
```
10.10.1.50 FILTER smtp:[10.100.0.35]:26,smtp:[10.100.0.36]:26
10.10.1.51 FILTER smtp:[10.100.0.31]:27,smtp:[10.100.0.32]:27
```

**`/etc/postfix/main.cf`** – Die `mynetworks`-Zeile wird aktualisiert:
```
mynetworks = 127.0.0.0/8 [::1]/128 10.10.1.50 10.10.1.51
```

Nach jeder Änderung werden `postmap` und `systemctl reload postfix` automatisch ausgeführt.

### Empfohlene main.cf-Einträge

```
smtpd_recipient_restrictions =
    permit_mynetworks,
    check_client_access hash:/etc/postfix/allowed_clients,
    reject_unauth_destination
```

---

## Deinstallation

```bash
sudo systemctl stop postfix-relay-manager
sudo systemctl disable postfix-relay-manager
sudo rm /etc/systemd/system/postfix-relay-manager.service
sudo systemctl daemon-reload
sudo rm -rf /opt/postfix-relay-manager
```
