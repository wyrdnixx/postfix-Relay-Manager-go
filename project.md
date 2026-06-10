# Postfix Relay Manager – Zusammenfassung app.js

## Zweck der Anwendung

Web-Oberfläche zur Verwaltung von Postfix-Relay-Clients. Ermöglicht es, Systeme (IP-Adressen) zu hinterlegen, die über zwei Postfix-Relay-Server E-Mails versenden dürfen. Schreibt automatisch Postfix-Konfigurationsdateien und lädt Postfix neu.

---

## Technische Basis (Node.js/Express)

| Komponente | Beschreibung |
|---|---|
| Framework | Express.js (HTTP-Server, Routing) |
| Sessions | express-session (Cookie-basiert, in-memory) |
| Passwort | SHA-256-Hash via Node `crypto`-Modul |
| Datenhaltung | Lokale JSON-Datei (`data.json`) – keine Datenbank |
| Port | 8080 |

---

## Konfiguration (Konstanten im Code)

```
username:           'admin'
passwordHash:       SHA-256-Hash von 'changeme'
relayServers:       ['10.100.0.31', '10.100.0.32']
allowedClientsFile: '/etc/postfix/allowed_clients'
mainCfFile:         '/etc/postfix/main.cf'
dataFile:           <Programmverzeichnis>/data.json
port:               8080
```

---

## Datenmodell (data.json)

```json
{
  "systems": [
    {
      "id": "1741600000000",
      "name": "Drucker EG",
      "ip": "192.168.1.100",
      "type": "internal"
    }
  ],
  "baseMynetworks": "127.0.0.0/8 [::ffff:127.0.0.0]/104",
  "allManagedIps": ["192.168.1.100", "10.0.0.5"]
}
```

- `id`: Unix-Timestamp in Millisekunden als String (eindeutige ID)
- `name`: Freitext, optional
- `ip`: IPv4-Adresse, auch CIDR-Notation erlaubt
- `type`: `"internal"` (Port 26) oder `"external"` (Port 27)
- `baseMynetworks`: Original-mynetworks-Wert aus main.cf beim ersten Speichern (ohne verwaltete IPs)
- `allManagedIps`: Alle jemals verwalteten IPs, auch gelöschte – verhindert, dass gelöschte IPs wieder in main.cf auftauchen

---

## Postfix-Konfigurationsdateien

### /etc/postfix/allowed_clients

Wird bei jeder Änderung komplett neu geschrieben. Eine Zeile pro System:

```
192.168.1.100 FILTER smtp:[10.100.0.31]:26,smtp:[10.100.0.32]:26
10.0.0.5      FILTER smtp:[10.100.0.31]:27,smtp:[10.100.0.32]:27
```

- Typ `internal` → Port **26** auf allen `relayServers`
- Typ `external` → Port **27** auf allen `relayServers`

### /etc/postfix/main.cf (mynetworks-Zeile)

Nur die `mynetworks`-Zeile wird verändert. Logik:
1. Beim ersten Speichern: Original-Wert von `mynetworks` lesen und als `baseMynetworks` in data.json sichern.
2. Bei jeder Änderung: Alle jemals verwalteten IPs aus `baseMynetworks` entfernen, dann nur aktuelle aktive IPs anhängen.

Ergebnis-Beispiel:
```
mynetworks = 127.0.0.0/8 [::ffff:127.0.0.0]/104 192.168.1.100 10.0.0.5
```

### Shell-Kommando nach jeder Änderung

```bash
# Als root:
postmap /etc/postfix/allowed_clients && systemctl reload postfix

# Als normaler User (sudo-Regel erforderlich):
sudo postmap /etc/postfix/allowed_clients && sudo systemctl reload postfix
```

Das Programm erkennt automatisch ob es als root läuft (`getuid() === 0`).

---

## HTTP-Routen

### Authentifizierung

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/login` | Login-Formular |
| POST | `/login` | SHA-256-Vergleich, bei Erfolg Session setzen und zu `/` weiterleiten |
| GET | `/logout` | Session zerstören, zu `/login` weiterleiten |

Alle anderen Routen sind durch Auth-Middleware geschützt: keine gültige Session → Redirect zu `/login`.

### Hauptseite

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/` | Zeigt Health-Karte (Relay-Server-Status) und Systemliste. Health-Status wird per JS asynchron nachgeladen. Flash-Meldung wird einmalig angezeigt und dann gelöscht. |

### System-Verwaltung

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/add` | Formular "System hinzufügen". Query-Parameter `?ip=X.X.X.X` füllt IP-Feld vor. |
| POST | `/add` | System speichern. Validierung: IP-Format, Typ, Duplikat-Check. Dann applyConfig, Flash setzen, Redirect zu `/`. |
| GET | `/edit/:id` | Formular "System bearbeiten" mit vorausgefüllten Feldern. |
| POST | `/edit/:id` | System aktualisieren. Gleiche Validierung wie beim Hinzufügen, aber Duplikat-Check prüft nur andere Systeme. |
| POST | `/delete/:id` | System löschen. Dann applyConfig, Flash setzen, Redirect zu `/`. |

Muster: Nach jeder Schreiboperation → applyConfig() → Flash-Meldung in Session → HTTP-Redirect zu `/` (POST–Redirect–GET).

### APIs (JSON-Antwort)

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/api/health` | Gibt Array mit TCP-Erreichbarkeit aller Relay-Server auf Port 26+27 zurück. |
| GET | `/api/logs` | Gibt gefilterte Deny-Log-Einträge als JSON zurück (für Auto-Refresh der Protokoll-Seite). |

JSON-Format `/api/health`:
```json
[
  { "server": "10.100.0.31", "port": 26, "label": "Intern", "ok": true },
  { "server": "10.100.0.31", "port": 27, "label": "Extern", "ok": false }
]
```

JSON-Format `/api/logs`:
```json
[
  {
    "ip": "10.108.12.200",
    "recipient": "user@example.com",
    "timeStr": "06.03.2026 10:23",
    "addUrl": "/add?ip=10.108.12.200"
  }
]
```

### Protokoll

| Methode | Pfad | Beschreibung |
|---|---|---|
| GET | `/logs` | Zeigt abgelehnte Relay-Versuche. Enthält Auto-Refresh-JavaScript (60 s). |

---

## Kernfunktionen

### buildAllowedClients(systems)
Erzeugt den Text-Inhalt der `allowed_clients`-Datei aus dem Systems-Array.

### readMynetworksLine()
Liest den aktuellen `mynetworks`-Wert aus `main.cf` per Regex `^mynetworks\s*=\s*(.*)$`.

### writeMainCf(data)
1. Falls `baseMynetworks` noch nicht gespeichert: jetzt lesen und speichern.
2. Set aller jemals verwalteten IPs (`allManagedIps` + aktuelle `systems.ip`) bilden.
3. `baseMynetworks` in Teile splitten, alle verwalteten IPs rausfiltern.
4. Aktuelle System-IPs anhängen.
5. `mynetworks`-Zeile in `main.cf` per Regex ersetzen (oder ans Ende anfügen, falls nicht vorhanden).

### applyConfig(data, callback)
1. `allowed_clients` neu schreiben.
2. `main.cf` neu schreiben.
3. Shell-Kommando ausführen: `postmap ... && systemctl reload postfix`.
4. Callback mit Fehler (oder null) aufrufen.

### checkPort(host, port, timeoutMs=3000)
TCP-Verbindungsversuch mit Timeout. Gibt Boolean zurück (true = erreichbar).

### checkRelayHealth()
Ruft `checkPort` für alle relayServers × Ports {26, 27} **parallel** auf. Gibt Array mit Ergebnissen zurück.

### getDeniedLog()
- Liest `/var/log/mail.log` (Fallback: `journalctl -u postfix`), maximal 100.000 Zeilen.
- Parst Zeilen, die `NOQUEUE: reject: RCPT from ... Relay access denied` enthalten.
- Regex-Capture: IP-Adresse und Empfänger-Adresse.
- Ergebnis-Array: `[{ ip, time, recipient }]`, maximal 200 Einträge, **neueste zuerst**.
- Ergebnis wird **60 Sekunden in einer globalen Variable gecacht**.

### parseLogTime(line, year)
Erkennt zwei Log-Timestamp-Formate:
1. **ISO 8601:** `2026-03-06T10:23:45` (RFC 5424, neuere rsyslog-Versionen)
2. **Klassisch:** `Mar  6 10:23:45` (RFC 3164, älteres Syslog)

Gibt Date-Objekt zurück oder `null` wenn kein Format erkannt.

### Log-Filterung in den Routen
In `/logs` und `/api/logs`: Einträge, deren IP bereits als System hinterlegt ist, werden **ausgeblendet** (nur unbekannte IPs anzeigen).

---

## HTML-Erzeugung

Kein Template-Engine. HTML wird als String in JavaScript-Funktionen erzeugt.

| Funktion | Beschreibung |
|---|---|
| `layout(title, body, flash)` | Gemeinsames Gerüst: DOCTYPE, Head, CSS, Header mit Navigation, main-Bereich |
| `loginPage(showError)` | Login-Seite mit eigenem minimalen Layout (kein Header) |
| `systemFormHtml(sys, action, label, errorMsg)` | Formular für Hinzufügen/Bearbeiten. `sys.ip` kann vorausgefüllt sein. |
| `esc(str)` | HTML-Escaping: `&`, `<`, `>`, `"` |
| `isValidIp(ip)` | Regex-Check: IPv4 oder IPv4/CIDR |
| `fmtTime(date)` | Formatiert Date zu `DD.MM.YYYY HH:MM` |

### CSS (inline im layout)
Einheitliches Design: dunkler Header (`#1a1a2e`), Card-Layout, Badge-Klassen (`badge-i`, `badge-e`, `badge-ok`, `badge-ko`), Button-Klassen (`btn-primary`, `btn-warn`, `btn-danger`, `btn-ghost`).

---

## Frontend-JavaScript (inline im HTML)

### Hauptseite
- `fetch('/api/health')` beim Laden → befüllt `#health-body` asynchron.
- Input-Event auf Suchfeld → filtert Zeilen in `#sys-tbody` per `textContent.includes()`.

### Protokoll-Seite
- `setInterval` alle 1 Sekunde: Countdown dekrementieren, bei 0 → `fetch('/api/logs')` → `#log-tbody` neu rendern.
- `renderRows(entries)`: Rendert JSON-Array zu HTML-Tabellenzeilen inkl. "Hinzufügen"-Button mit Link zu `/add?ip=<IP>`.

---

## Session & Flash-Meldungen

- Sessions: in-memory, gehen beim Neustart verloren.
- Flash-Meldungen: in Session gespeichert, beim nächsten GET `/` einmalig angezeigt und gelöscht.
- Flash-Typen: `ok` (grüner Hintergrund) oder `err` (roter Hintergrund).

---

## Validierungsregeln

| Feld | Regel |
|---|---|
| IP | Regex `^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$` |
| Typ | Muss exakt `"internal"` oder `"external"` sein |
| Duplikat (Hinzufügen) | IP darf noch nicht in `systems` existieren |
| Duplikat (Bearbeiten) | IP darf nicht von einem **anderen** System verwendet werden |

---

## Deployment

Läuft als systemd-Service direkt auf Port 8080. Kein Webserver (nginx/apache) vorgelagert. Benötigt Schreibrechte auf `/etc/postfix/allowed_clients` und `/etc/postfix/main.cf` sowie Ausführungsrechte für `postmap` und `systemctl reload postfix` (via sudo oder root).
