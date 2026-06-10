package main

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

// ─── Hilfsfunktionen ─────────────────────────────────────────────────────────

func esc(s string) string { return html.EscapeString(s) }

func nameOrDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

// ipSortKey wandelt eine IPv4-Adresse in einen lückenlos sortierbaren String
// um (jedes Oktett auf 3 Stellen aufgefüllt). CIDR-Ranges bleiben unverändert.
func ipSortKey(ip string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	var buf strings.Builder
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return ip
		}
		if i > 0 {
			buf.WriteByte('.')
		}
		fmt.Fprintf(&buf, "%03d", n)
	}
	return buf.String()
}

// relayPort gibt den Port des ersten Relay-Servers zurück (oder fallback).
func relayPort(servers []RelayServer, fallback int) int {
	if len(servers) > 0 {
		return servers[0].Port
	}
	return fallback
}

// ─── Kategorie-Metadaten ──────────────────────────────────────────────────────

var categoryLabel = map[string]string{
	"printer": "Drucker",
	"server":  "Server",
	"scanner": "Scanner",
	"network": "Netzwerk",
	"other":   "Sonstiges",
	"":        "Sonstiges",
}

var categoryCss = map[string]string{
	"printer": "badge-cat-printer",
	"server":  "badge-cat-server",
	"scanner": "badge-cat-scanner",
	"network": "badge-cat-network",
	"other":   "badge-cat-other",
	"":        "badge-cat-other",
}

var categoryIcon = map[string]string{
	"printer": "🖨",
	"server":  "🖥",
	"scanner": "📠",
	"network": "🔌",
	"other":   "📦",
	"":        "📦",
}

func catKey(c string) string {
	if c == "" {
		return "other"
	}
	return c
}

// ─── CSS ─────────────────────────────────────────────────────────────────────

func css() string {
	return `
    *,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
    body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f0f2f5;color:#222}
    header{background:#1a1a2e;color:#fff;padding:14px 32px;display:flex;justify-content:space-between;align-items:center}
    header h1{font-size:1.1rem;font-weight:600;letter-spacing:.02em}
    header nav{display:flex;gap:20px;align-items:center}
    header a{color:#aaa;text-decoration:none;font-size:.85rem}
    header a:hover{color:#fff}
    main{max-width:1040px;margin:32px auto;padding:0 16px}
    .card{background:#fff;border-radius:8px;box-shadow:0 1px 4px rgba(0,0,0,.1);padding:24px;margin-bottom:24px}
    h2{font-size:1rem;font-weight:600;margin-bottom:16px;color:#1a1a2e}
    table{width:100%;border-collapse:collapse}
    th{text-align:left;padding:9px 12px;background:#f7f8fa;font-size:.8rem;color:#555;border-bottom:1px solid #e0e0e0;font-weight:600;white-space:nowrap}
    th.sortable{cursor:pointer;user-select:none}
    th.sortable:hover{background:#eff0f2;color:#1a1a2e}
    td{padding:10px 12px;border-bottom:1px solid #f0f0f0;font-size:.88rem;vertical-align:middle}
    tr:last-child td{border-bottom:none}
    code{font-family:'SFMono-Regular',Consolas,monospace;background:#f4f4f4;padding:2px 6px;border-radius:4px;font-size:.84rem}
    .badge{display:inline-block;padding:2px 10px;border-radius:12px;font-size:.75rem;font-weight:600}
    .badge-i{background:#e3f2fd;color:#1565c0}
    .badge-e{background:#fce4ec;color:#c62828}
    .badge-ok{background:#e8f5e9;color:#2e7d32}
    .badge-ko{background:#ffebee;color:#c62828}
    .badge-warn{background:#fff8e1;color:#e65100}
    .badge-cat-printer{background:#fff7ed;color:#c2410c}
    .badge-cat-server{background:#eff6ff;color:#1d4ed8}
    .badge-cat-scanner{background:#f0fdf4;color:#15803d}
    .badge-cat-network{background:#faf5ff;color:#7e22ce}
    .badge-cat-other{background:#f9fafb;color:#374151}
    .btn{display:inline-block;padding:7px 16px;border-radius:6px;font-size:.85rem;cursor:pointer;border:none;text-decoration:none;font-weight:500;line-height:1.4}
    .btn-primary{background:#1a1a2e;color:#fff}
    .btn-primary:hover{background:#2d2d4e}
    .btn-warn{background:#f57c00;color:#fff}
    .btn-warn:hover{background:#e65100}
    .btn-danger{background:#e53935;color:#fff}
    .btn-danger:hover{background:#b71c1c}
    .btn-ghost{background:#f0f0f0;color:#444}
    .btn-ghost:hover{background:#ddd}
    .btn-preview{background:#e0f2fe;color:#0369a1;border:1px solid #bae6fd}
    .btn-preview:hover{background:#bae6fe;color:#0c4a6e}
    .actions{display:flex;gap:8px;flex-wrap:wrap;align-items:center}
    .form-row{margin-bottom:16px}
    label{display:block;font-size:.82rem;font-weight:600;margin-bottom:5px;color:#555}
    input[type=text],input[type=password],select{width:100%;padding:9px 12px;border:1px solid #d0d0d0;border-radius:6px;font-size:.9rem;background:#fff}
    input:focus,select:focus{outline:none;border-color:#1a1a2e;box-shadow:0 0 0 2px rgba(26,26,46,.12)}
    .alert{padding:11px 16px;border-radius:6px;margin-bottom:16px;font-size:.88rem}
    .alert-ok{background:#e8f5e9;color:#2e7d32;border:1px solid #a5d6a7}
    .alert-err{background:#ffebee;color:#c62828;border:1px solid #ef9a9a}
    .empty{text-align:center;color:#aaa;padding:32px;font-size:.9rem}
    .toolbar{display:flex;align-items:center;justify-content:space-between;margin-bottom:12px;flex-wrap:wrap;gap:8px}
    .cat-filters{display:flex;gap:6px;flex-wrap:wrap;margin-bottom:12px}
    .cat-btn{padding:4px 13px;border-radius:20px;border:1px solid #e0e0e0;background:#fff;cursor:pointer;font-size:.8rem;color:#555;transition:all .15s}
    .cat-btn.active,.cat-btn:hover{background:#1a1a2e;color:#fff;border-color:#1a1a2e}
    .ip-hn{font-size:.74rem;color:#888;margin-top:2px;font-family:'SFMono-Regular',Consolas,monospace;display:none}
    .sort-icon{opacity:.4;font-size:.8em;margin-left:3px}
    th.sortable:hover .sort-icon{opacity:.8}
    .modal-overlay{display:none;position:fixed;inset:0;background:rgba(0,0,0,.5);z-index:200;align-items:flex-start;justify-content:center;padding:40px 16px;overflow-y:auto}
    .modal-box{background:#fff;border-radius:10px;padding:28px;max-width:720px;width:100%;box-shadow:0 8px 32px rgba(0,0,0,.2);position:relative}
    .modal-box h3{font-size:1rem;font-weight:600;margin-bottom:16px;color:#1a1a2e}
    .modal-box pre{background:#1a1a2e;color:#e2e8f0;padding:14px 16px;border-radius:6px;font-size:.82rem;overflow-x:auto;margin-bottom:12px;white-space:pre-wrap;word-break:break-all}
    .modal-box .pre-label{font-size:.75rem;font-weight:600;color:#888;margin-bottom:4px}
    .modal-close{position:absolute;top:16px;right:20px;font-size:1.4rem;cursor:pointer;color:#aaa;line-height:1}
    .modal-close:hover{color:#333}
    .queue-pill{display:inline-flex;align-items:center;gap:5px;padding:3px 11px;border-radius:12px;font-size:.75rem;font-weight:700;text-decoration:none;transition:opacity .15s}
    .queue-pill:hover{opacity:.8}
    .queue-pill-ok{background:rgba(46,125,50,.25);color:#a5d6a7}
    .queue-pill-warn{background:rgba(230,81,0,.3);color:#ffcc80}
    .queue-pill-err{background:rgba(198,40,40,.35);color:#ef9a9a}
    .queue-pill-unknown{background:rgba(255,255,255,.1);color:#888}
    .status-pre{background:#1a1a2e;color:#e2e8f0;padding:14px 16px;border-radius:6px;font-size:.8rem;overflow-x:auto;white-space:pre-wrap;word-break:break-all;font-family:'SFMono-Regular',Consolas,monospace;margin-top:6px}
  `
}

// ─── Gemeinsames Layout ───────────────────────────────────────────────────────

func layout(title, body, flashHTML string) string {
	queueBadge := queueNavBadge()
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="de">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>%s – Postfix Relay Manager</title>
  <style>%s</style>
</head>
<body>
  <header>
    <h1><a href="/" style="color:inherit;text-decoration:none">Postfix Relay Manager</a></h1>
    <nav>
      %s
      <a href="/syscheck">Systemprüfung</a>
      <a href="/logs">Protokoll</a>
      <a href="/settings">Einstellungen</a>
      <a href="/logout">Abmelden</a>
    </nav>
  </header>
  <main>
    %s
    %s
  </main>
</body>
</html>`, esc(title), css(), queueBadge, flashHTML, body)
}

func queueNavBadge() string {
	n := postfixQueueSize()
	switch {
	case n < 0:
		return `<a href="/postfix" class="queue-pill queue-pill-unknown" title="Warteschlange nicht lesbar">? Mails</a>`
	case n == 0:
		return `<a href="/postfix" class="queue-pill queue-pill-ok" title="Warteschlange leer">✓ Queue leer</a>`
	case n < 10:
		return fmt.Sprintf(`<a href="/postfix" class="queue-pill queue-pill-warn" title="Mails in Warteschlange">⚠ %d Mail(s)</a>`, n)
	default:
		return fmt.Sprintf(`<a href="/postfix" class="queue-pill queue-pill-err" title="Viele Mails in Warteschlange">✗ %d Mails</a>`, n)
	}
}

func flashToHTML(f *Flash) string {
	if f == nil {
		return ""
	}
	cls := "alert-ok"
	if f.Type == "err" {
		cls = "alert-err"
	}
	return fmt.Sprintf(`<div class="alert %s">%s</div>`, cls, esc(f.Msg))
}

// ─── Login-Seite ─────────────────────────────────────────────────────────────

func loginPage(showError bool) string {
	errPart := ""
	if showError {
		errPart = `<div class="err">Ungültige Anmeldedaten.</div>`
	}
	return `<!DOCTYPE html>
<html lang="de">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Anmelden – Postfix Relay Manager</title>
  <style>
    *,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
    body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
         background:#f0f2f5;display:flex;align-items:center;justify-content:center;min-height:100vh}
    .card{background:#fff;border-radius:10px;box-shadow:0 2px 12px rgba(0,0,0,.12);
          padding:40px 36px;width:340px}
    h1{font-size:1.1rem;margin-bottom:6px;color:#1a1a2e;text-align:center;font-weight:600}
    .sub{text-align:center;color:#888;font-size:.8rem;margin-bottom:24px}
    .row{margin-bottom:14px}
    label{display:block;font-size:.82rem;font-weight:600;margin-bottom:5px;color:#555}
    input{width:100%;padding:9px 12px;border:1px solid #d0d0d0;border-radius:6px;font-size:.9rem}
    input:focus{outline:none;border-color:#1a1a2e;box-shadow:0 0 0 2px rgba(26,26,46,.12)}
    .btn{width:100%;padding:10px;border:none;border-radius:6px;background:#1a1a2e;
         color:#fff;font-size:.95rem;cursor:pointer;margin-top:8px;font-weight:500}
    .btn:hover{background:#2d2d4e}
    .err{padding:10px 14px;border-radius:6px;margin-bottom:14px;font-size:.85rem;
         background:#ffebee;color:#c62828;border:1px solid #ef9a9a}
  </style>
</head>
<body>
  <div class="card">
    <h1>Postfix Relay Manager</h1>
    <p class="sub">Bitte anmelden</p>
    ` + errPart + `
    <form method="POST" action="/login">
      <div class="row">
        <label>Benutzername</label>
        <input type="text" name="username" required autofocus autocomplete="username">
      </div>
      <div class="row">
        <label>Passwort</label>
        <input type="password" name="password" required autocomplete="current-password">
      </div>
      <button class="btn" type="submit">Anmelden</button>
    </form>
  </div>
</body>
</html>`
}

// ─── Übersichtsseite ─────────────────────────────────────────────────────────

func indexPage(systems []System, flash *Flash) string {
	typeLabel := map[string]string{"internal": "Intern", "external": "Extern"}
	typeCss := map[string]string{"internal": "badge badge-i", "external": "badge badge-e"}

	var rows strings.Builder
	if len(systems) == 0 {
		rows.WriteString(`<tr><td colspan="5" class="empty">Keine Systeme konfiguriert.</td></tr>`)
	} else {
		for _, s := range systems {
			cat := catKey(s.Category)
			catLbl := categoryLabel[s.Category]
			catCss := categoryCss[s.Category]
			catIco := categoryIcon[s.Category]
			typLbl := typeLabel[s.Type]
			typCss := typeCss[s.Type]

			fmt.Fprintf(&rows,
				`<tr data-category="%s">
        <td data-sort="%s">%s</td>
        <td data-sort="%s"><span class="badge %s">%s %s</span></td>
        <td data-sort="%s">
          <code>%s</code>
          <div class="ip-hn" data-ip="%s"></div>
        </td>
        <td><span class="%s">%s</span></td>
        <td class="actions">
          <a href="/edit/%s" class="btn btn-warn">Bearbeiten</a>
          <form method="POST" action="/delete/%s"
                onsubmit="return confirm('System %s wirklich löschen?')">
            <button type="submit" class="btn btn-danger">Löschen</button>
          </form>
        </td>
      </tr>`,
				esc(cat),
				esc(nameOrDash(s.Name)),
				esc(nameOrDash(s.Name)),
				esc(catLbl),
				esc(catCss),
				catIco,
				esc(catLbl),
				ipSortKey(s.IP),
				esc(s.IP),
				esc(s.IP),
				esc(typCss),
				esc(typLbl),
				esc(s.ID),
				esc(s.ID),
				esc(s.IP),
			)
		}
	}

	body := fmt.Sprintf(`
<div class="card">
  <h2>Relay-Server Status</h2>
  <table>
    <thead><tr><th>Server</th><th>Port</th><th>Status</th></tr></thead>
    <tbody id="health-body">
      <tr><td colspan="3" style="text-align:center;color:#aaa;padding:16px;font-size:.88rem">Wird geprüft...</td></tr>
    </tbody>
  </table>
</div>

<div class="card">
  <div class="toolbar">
    <h2>Zugelassene Systeme (%d)</h2>
    <div style="display:flex;gap:10px;align-items:center">
      <input type="search" id="sys-search" placeholder="Name, IP oder Hostname suchen..."
             style="padding:7px 12px;border:1px solid #d0d0d0;border-radius:6px;font-size:.85rem;width:240px">
      <a href="/bulk-add" class="btn btn-preview">↑ Bulk-Import</a>
      <a href="/add" class="btn btn-primary">+ System hinzufügen</a>
    </div>
  </div>

  <div class="cat-filters">
    <button class="cat-btn active" onclick="filterCat('')" data-cat="">Alle</button>
    <button class="cat-btn" onclick="filterCat('printer')" data-cat="printer">🖨 Drucker</button>
    <button class="cat-btn" onclick="filterCat('server')" data-cat="server">🖥 Server</button>
    <button class="cat-btn" onclick="filterCat('scanner')" data-cat="scanner">📠 Scanner</button>
    <button class="cat-btn" onclick="filterCat('network')" data-cat="network">🔌 Netzwerk</button>
    <button class="cat-btn" onclick="filterCat('other')" data-cat="other">📦 Sonstiges</button>
  </div>

  <table>
    <thead>
      <tr>
        <th class="sortable" onclick="sortTable(0)">Bezeichnung <span class="sort-icon" id="si-0">⇅</span></th>
        <th class="sortable" onclick="sortTable(1)">Kategorie <span class="sort-icon" id="si-1">⇅</span></th>
        <th class="sortable" onclick="sortTable(2)">IP-Adresse <span class="sort-icon" id="si-2">⇅</span></th>
        <th class="sortable" onclick="sortTable(3)">Versandtyp <span class="sort-icon" id="si-3">⇅</span></th>
        <th>Aktionen</th>
      </tr>
    </thead>
    <tbody id="sys-tbody">%s</tbody>
  </table>
</div>

<script>
// ── Health ──
fetch('/api/health')
  .then(r => r.json())
  .then(health => {
    document.getElementById('health-body').innerHTML = health.map(h =>
      '<tr><td><code>' + h.server + '</code></td>' +
      '<td>Port ' + h.port + ' (' + h.label + ')</td>' +
      '<td><span class="badge ' + (h.ok ? 'badge-ok' : 'badge-ko') + '">' +
      (h.ok ? 'Erreichbar' : 'Nicht erreichbar') + '</span></td></tr>'
    ).join('');
  })
  .catch(() => {
    document.getElementById('health-body').innerHTML =
      '<tr><td colspan="3" style="text-align:center;color:#c62828;padding:16px;font-size:.88rem">Fehler beim Laden</td></tr>';
  });

// ── Hostname-Auflösung ──
fetch('/api/resolve')
  .then(r => r.json())
  .then(hosts => {
    document.querySelectorAll('.ip-hn[data-ip]').forEach(el => {
      const hn = hosts[el.dataset.ip];
      if (hn) { el.textContent = hn; el.style.display = 'block'; }
    });
  })
  .catch(() => {});

// ── Sortierung ──
let sortCol = -1, sortAsc = true;
function sortTable(col) {
  if (sortCol === col) sortAsc = !sortAsc;
  else { sortCol = col; sortAsc = true; }

  const tbody = document.getElementById('sys-tbody');
  const rows = Array.from(tbody.querySelectorAll('tr'));
  rows.sort((a, b) => {
    const av = a.cells[col]?.dataset.sort ?? a.cells[col]?.textContent.trim() ?? '';
    const bv = b.cells[col]?.dataset.sort ?? b.cells[col]?.textContent.trim() ?? '';
    return av.localeCompare(bv, 'de', {numeric: false}) * (sortAsc ? 1 : -1);
  });
  rows.forEach(r => tbody.appendChild(r));

  for (let i = 0; i < 4; i++) {
    const el = document.getElementById('si-' + i);
    if (el) el.textContent = i === col ? (sortAsc ? '↑' : '↓') : '⇅';
  }
  applyFilters();
}

// ── Filter ──
let activeCategory = '';
function filterCat(cat) {
  activeCategory = cat;
  document.querySelectorAll('.cat-btn').forEach(b =>
    b.classList.toggle('active', b.dataset.cat === cat));
  applyFilters();
}
function applyFilters() {
  const q = (document.getElementById('sys-search').value || '').toLowerCase();
  document.querySelectorAll('#sys-tbody tr').forEach(row => {
    const matchText = row.textContent.toLowerCase().includes(q);
    const matchCat = !activeCategory || row.dataset.category === activeCategory;
    row.style.display = (matchText && matchCat) ? '' : 'none';
  });
}
document.getElementById('sys-search').addEventListener('input', applyFilters);
</script>`, len(systems), rows.String())

	return layout("Übersicht", body, flashToHTML(flash))
}

// ─── Formular: System hinzufügen / bearbeiten ─────────────────────────────────

func systemFormPage(sys System, action, pageTitle, btnLabel, errMsg string) string {
	internalPort, externalPort := relayPort(relayServersInternal, 26), relayPort(relayServersExternal, 27)

	selInt, selExt := "", ""
	if sys.Type == "external" {
		selExt = " selected"
	} else {
		selInt = " selected"
	}

	cats := []struct{ val, label string }{
		{"printer", "🖨 Drucker"},
		{"server", "🖥 Server"},
		{"scanner", "📠 Scanner"},
		{"network", "🔌 Netzwerk"},
		{"other", "📦 Sonstiges"},
	}
	var catOptions strings.Builder
	curCat := catKey(sys.Category)
	for _, c := range cats {
		sel := ""
		if c.val == curCat {
			sel = " selected"
		}
		fmt.Fprintf(&catOptions, `<option value="%s"%s>%s</option>`, c.val, sel, c.label)
	}

	errHTML := ""
	if errMsg != "" {
		errHTML = fmt.Sprintf(`<div class="alert alert-err">%s</div>`, esc(errMsg))
	}

	body := fmt.Sprintf(`
<div class="card">
  <h2>System konfigurieren</h2>
  %s
  <form method="POST" action="%s" id="sys-form">
    <input type="hidden" name="sys_id" value="%s">
    <div class="form-row">
      <label>Bezeichnung / Name <span style="font-weight:400;color:#aaa">(optional)</span></label>
      <input type="text" name="name" value="%s" placeholder="z.B. Drucker EG, Scanner Lager">
    </div>
    <div class="form-row">
      <label>IP-Adresse *</label>
      <input type="text" name="ip" id="f-ip" value="%s" placeholder="192.168.1.100" required>
    </div>
    <div class="form-row">
      <label>Versandtyp *</label>
      <select name="type" id="f-type" required>
        <option value="internal"%s>Intern (Port %d)</option>
        <option value="external"%s>Extern (Port %d)</option>
      </select>
    </div>
    <div class="form-row">
      <label>Kategorie</label>
      <select name="category" id="f-cat">%s</select>
    </div>
    <div class="actions" style="margin-top:8px">
      <button type="submit" class="btn btn-primary">%s</button>
      <button type="button" class="btn btn-preview" onclick="showPreview()">Vorschau</button>
      <a href="/" class="btn btn-ghost">Abbrechen</a>
    </div>
  </form>
</div>

<!-- Vorschau-Modal -->
<div class="modal-overlay" id="preview-modal" onclick="if(event.target===this)closeModal()">
  <div class="modal-box">
    <span class="modal-close" onclick="closeModal()">×</span>
    <h3>Konfigurationsvorschau</h3>
    <p style="font-size:.85rem;color:#666;margin-bottom:16px">
      So würden die Postfix-Konfigurationsdateien nach dem Speichern aussehen:
    </p>
    <div class="pre-label">/etc/postfix/allowed_clients</div>
    <pre id="prev-ac">–</pre>
    <div class="pre-label">mynetworks-Zeile in /etc/postfix/main.cf</div>
    <pre id="prev-mn">–</pre>
    <div style="text-align:right">
      <button class="btn btn-ghost" onclick="closeModal()">Schließen</button>
    </div>
  </div>
</div>

<script>
function closeModal() {
  document.getElementById('preview-modal').style.display = 'none';
}
async function showPreview() {
  const ip = document.getElementById('f-ip').value.trim();
  if (!ip) { alert('Bitte zuerst eine IP-Adresse eingeben.'); return; }
  const type = document.getElementById('f-type').value;
  const id   = document.querySelector('[name=sys_id]').value;
  try {
    const body = new URLSearchParams({ip, type, sys_id: id});
    const res  = await fetch('/api/preview', {method: 'POST', body});
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const d = await res.json();
    document.getElementById('prev-ac').textContent = d.allowedClients || '(leer)';
    document.getElementById('prev-mn').textContent = d.mynetworks    || '(leer)';
    document.getElementById('preview-modal').style.display = 'flex';
  } catch(e) {
    alert('Vorschau konnte nicht geladen werden: ' + e.message);
  }
}
</script>`,
		errHTML,
		esc(action),
		esc(sys.ID),
		esc(sys.Name),
		esc(sys.IP),
		selInt, internalPort,
		selExt, externalPort,
		catOptions.String(),
		esc(btnLabel),
	)

	return layout(pageTitle, body, "")
}

// ─── Protokoll-Seite ──────────────────────────────────────────────────────────

func logsPage(entries []LogEntry) string {
	var rows strings.Builder
	if len(entries) == 0 {
		rows.WriteString(`<tr><td colspan="4" class="empty">Keine unbekannten abgelehnten Verbindungen gefunden.</td></tr>`)
	} else {
		for _, e := range entries {
			fmt.Fprintf(&rows,
				`<tr>
        <td style="white-space:nowrap">%s</td>
        <td><code>%s</code></td>
        <td>%s</td>
        <td><a href="%s" class="btn btn-primary" style="font-size:.78rem;padding:4px 10px">Hinzufügen</a></td>
      </tr>`,
				esc(e.TimeStr),
				esc(e.IP),
				esc(e.Recipient),
				"/add?ip="+url.QueryEscape(e.IP),
			)
		}
	}

	body := fmt.Sprintf(`
<div class="card">
  <div class="toolbar">
    <h2>Abgelehnte Relay-Versuche <span style="font-weight:400;color:#aaa">(letzte 200, nur unbekannte IPs)</span></h2>
    <div style="display:flex;gap:12px;align-items:center">
      <span id="countdown" style="font-size:.8rem;color:#aaa"></span>
      <a href="/" class="btn btn-ghost">Zur Übersicht</a>
    </div>
  </div>
  <table>
    <thead>
      <tr><th>Zeitpunkt</th><th>IP-Adresse</th><th>Empfänger</th><th></th></tr>
    </thead>
    <tbody id="log-tbody">%s</tbody>
  </table>
</div>
<script>
function renderRows(entries) {
  if (!entries || !entries.length)
    return '<tr><td colspan="4" class="empty">Keine unbekannten abgelehnten Verbindungen.</td></tr>';
  return entries.map(d =>
    '<tr><td style="white-space:nowrap">' + d.timeStr + '</td>' +
    '<td><code>' + d.ip + '</code></td>' +
    '<td>' + d.recipient + '</td>' +
    '<td><a href="' + d.addUrl + '" class="btn btn-primary" style="font-size:.78rem;padding:4px 10px">Hinzufügen</a></td></tr>'
  ).join('');
}
function refresh() {
  fetch('/api/logs').then(r => r.json())
    .then(d => { document.getElementById('log-tbody').innerHTML = renderRows(d); })
    .catch(() => {});
}
let secs = 60;
const cdEl = document.getElementById('countdown');
setInterval(() => {
  secs--;
  if (secs <= 0) { secs = 60; refresh(); }
  cdEl.textContent = 'Aktualisierung in ' + secs + ' s';
}, 1000);
cdEl.textContent = 'Aktualisierung in ' + secs + ' s';
</script>`, rows.String())

	return layout("Protokoll", body, "")
}

// ─── Bulk-Import-Seite ────────────────────────────────────────────────────────

func bulkAddPage(errMsg string) string {
	internalPort, externalPort := relayPort(relayServersInternal, 26), relayPort(relayServersExternal, 27)

	cats := []struct{ val, label string }{
		{"printer", "🖨 Drucker"}, {"server", "🖥 Server"},
		{"scanner", "📠 Scanner"}, {"network", "🔌 Netzwerk"}, {"other", "📦 Sonstiges"},
	}
	var catOpts strings.Builder
	for _, c := range cats {
		sel := ""
		if c.val == "other" {
			sel = " selected"
		}
		fmt.Fprintf(&catOpts, `<option value="%s"%s>%s</option>`, c.val, sel, c.label)
	}

	errHTML := ""
	if errMsg != "" {
		errHTML = fmt.Sprintf(`<div class="alert alert-err">%s</div>`, esc(errMsg))
	}

	body := fmt.Sprintf(`
<div class="card" style="max-width:640px">
  <h2>Bulk-Import: Mehrere IPs auf einmal hinzufügen</h2>
  %s
  <div class="alert alert-ok" style="margin-bottom:20px;font-size:.85rem">
    <strong>Tipp für Exchange-Empfangskonnektoren:</strong><br>
    EAC → Nachrichtenfluss → Empfangskonnektoren → Konnektor bearbeiten → Sicherheit → Remote IP-Bereiche.<br>
    Alle IPs markieren, kopieren und unten einfügen.
  </div>
  <form method="POST" action="/bulk-add">
    <div class="form-row">
      <label>IP-Adressen <span style="font-weight:400;color:#aaa">(eine pro Zeile oder kommagetrennt, # = Kommentar)</span></label>
      <textarea name="ips" rows="12" required
        style="width:100%%;padding:9px 12px;border:1px solid #d0d0d0;border-radius:6px;
               font-family:'SFMono-Regular',Consolas,monospace;font-size:.88rem;resize:vertical"
        placeholder="# Exchange Empfangskonnektor Extern&#10;185.12.44.10&#10;185.12.44.11&#10;10.20.5.0/24&#10;&#10;# Niederlassung Wien&#10;10.30.1.55&#10;10.30.1.56"></textarea>
    </div>
    <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px">
      <div class="form-row" style="margin-bottom:0">
        <label>Bezeichnung <span style="font-weight:400;color:#aaa">(optional, für alle)</span></label>
        <input type="text" name="name" placeholder="z.B. Exchange Ext">
      </div>
      <div class="form-row" style="margin-bottom:0">
        <label>Versandtyp *</label>
        <select name="type" required>
          <option value="internal">Intern (Port %d)</option>
          <option value="external" selected>Extern (Port %d)</option>
        </select>
      </div>
      <div class="form-row" style="margin-bottom:0">
        <label>Kategorie</label>
        <select name="category">%s</select>
      </div>
    </div>
    <div class="actions" style="margin-top:16px">
      <button type="submit" class="btn btn-primary">IPs importieren</button>
      <a href="/" class="btn btn-ghost">Abbrechen</a>
    </div>
  </form>
</div>`, errHTML, internalPort, externalPort, catOpts.String())

	return layout("Bulk-Import", body, "")
}

func bulkResultPage(r *BulkResult) string {
	var sb strings.Builder

	sb.WriteString(`<div class="card" style="max-width:640px"><h2>Import-Ergebnis</h2>`)

	// Erfolgreich hinzugefügt
	if len(r.Added) > 0 {
		fmt.Fprintf(&sb, `<div style="margin-bottom:16px">
      <div style="font-weight:600;color:#2e7d32;margin-bottom:6px">✅ %d IP(s) erfolgreich hinzugefügt</div>
      <div style="font-family:monospace;font-size:.84rem;background:#f0fdf4;border:1px solid #a5d6a7;
                  border-radius:6px;padding:10px 14px;line-height:1.8">%s</div></div>`,
			len(r.Added), strings.Join(r.Added, "<br>"))
	}

	// Bereits vorhanden
	if len(r.Skipped) > 0 {
		fmt.Fprintf(&sb, `<div style="margin-bottom:16px">
      <div style="font-weight:600;color:#b45309;margin-bottom:6px">⚠️ %d IP(s) übersprungen (bereits vorhanden)</div>
      <div style="font-family:monospace;font-size:.84rem;background:#fffbeb;border:1px solid #fcd34d;
                  border-radius:6px;padding:10px 14px;line-height:1.8">%s</div></div>`,
			len(r.Skipped), strings.Join(r.Skipped, "<br>"))
	}

	// Ungültig
	if len(r.Invalid) > 0 {
		fmt.Fprintf(&sb, `<div style="margin-bottom:16px">
      <div style="font-weight:600;color:#c62828;margin-bottom:6px">❌ %d Eingabe(n) ungültig (ignoriert)</div>
      <div style="font-family:monospace;font-size:.84rem;background:#ffebee;border:1px solid #ef9a9a;
                  border-radius:6px;padding:10px 14px;line-height:1.8">%s</div>
      <div style="font-size:.8rem;color:#888;margin-top:4px">
        Nur IPv4-Adressen und CIDR-Notation (z.B. 10.0.0.0/24) werden unterstützt.
        Exchange-Bereiche im Format "x.x.x.x-x.x.x.x" bitte in CIDR umrechnen.
      </div></div>`,
			len(r.Invalid), strings.Join(r.Invalid, "<br>"))
	}

	// Postfix-Fehler
	if r.ApplyErr != nil {
		fmt.Fprintf(&sb,
			`<div class="alert alert-err">Postfix-Reload fehlgeschlagen: %s</div>`,
			esc(r.ApplyErr.Error()))
	} else if len(r.Added) > 0 {
		sb.WriteString(`<div class="alert alert-ok">Postfix wurde neu geladen.</div>`)
	}

	sb.WriteString(`<div class="actions" style="margin-top:8px">
    <a href="/" class="btn btn-primary">Zur Übersicht</a>
    <a href="/bulk-add" class="btn btn-ghost">Weitere IPs importieren</a>
  </div></div>`)

	return layout("Import-Ergebnis", sb.String(), "")
}

// ─── Einstellungen-Seite ──────────────────────────────────────────────────────

func settingsPage(flash *Flash) string {
	body := `
<div class="card" style="max-width:480px">
  <h2>Passwort ändern</h2>
  <form method="POST" action="/settings">
    <div class="form-row">
      <label>Aktuelles Passwort</label>
      <input type="password" name="current_password" required autocomplete="current-password">
    </div>
    <div class="form-row">
      <label>Neues Passwort <span style="font-weight:400;color:#aaa">(mindestens 8 Zeichen)</span></label>
      <input type="password" name="new_password" required minlength="8" autocomplete="new-password">
    </div>
    <div class="form-row">
      <label>Neues Passwort bestätigen</label>
      <input type="password" name="confirm_password" required autocomplete="new-password">
    </div>
    <div class="actions" style="margin-top:8px">
      <button type="submit" class="btn btn-primary">Passwort ändern</button>
      <a href="/" class="btn btn-ghost">Abbrechen</a>
    </div>
  </form>
</div>
<div class="card" style="max-width:480px;margin-top:0">
  <h2>Hinweise</h2>
  <ul style="font-size:.88rem;line-height:1.8;color:#555;padding-left:1.2em">
    <li>Das neue Passwort wird dauerhaft in <code>data.json</code> gespeichert.</li>
    <li>Nach einem Neustart ohne <code>data.json</code> greift wieder das Standard-Passwort aus <code>config.go</code>.</li>
    <li>Aktive Sitzungen bleiben bis zum nächsten Neustart gültig.</li>
  </ul>
</div>`
	return layout("Einstellungen", body, flashToHTML(flash))
}

// ─── Systemprüfungs-Seite ─────────────────────────────────────────────────────

func sysCheckPage(results []CheckResult) string {
	statusIcon := map[string]string{
		"ok":   "✓",
		"warn": "⚠",
		"err":  "✗",
	}
	statusBadge := map[string]string{
		"ok":   "badge-ok",
		"warn": "badge-warn",
		"err":  "badge-ko",
	}
	statusLabel := map[string]string{
		"ok":   "OK",
		"warn": "Warnung",
		"err":  "Fehler",
	}

	allOK := true
	for _, r := range results {
		if r.Status != "ok" {
			allOK = false
			break
		}
	}

	var rows strings.Builder
	for _, r := range results {
		icon := statusIcon[r.Status]
		badge := statusBadge[r.Status]
		label := statusLabel[r.Status]

		detailHTML := ""
		if r.Detail != "" {
			detailHTML = fmt.Sprintf(
				`<div style="margin-top:6px;font-size:.82rem;color:#666;background:#f7f8fa;border-radius:5px;padding:8px 12px;white-space:pre-wrap;font-family:'SFMono-Regular',Consolas,monospace">%s</div>`,
				esc(r.Detail),
			)
		}

		fmt.Fprintf(&rows,
			`<tr>
      <td style="white-space:nowrap;font-weight:500">%s</td>
      <td><span class="badge %s" style="min-width:70px;text-align:center">%s %s</span></td>
      <td>
        <code>%s</code>
        %s
      </td>
    </tr>`,
			esc(r.Name),
			badge, icon, label,
			esc(r.Message),
			detailHTML,
		)
	}

	summaryHTML := `<div class="alert alert-ok" style="margin-bottom:20px">Alle Prüfungen bestanden – Postfix ist korrekt konfiguriert.</div>`
	if !allOK {
		summaryHTML = `<div class="alert alert-err" style="margin-bottom:20px">Einige Prüfungen schlugen fehl. Beheben Sie die markierten Probleme, bevor Sie den Relay Manager verwenden.</div>`
	}

	body := fmt.Sprintf(`
<div class="card">
  <div class="toolbar">
    <h2>Systemprüfung</h2>
    <form method="GET" action="/syscheck">
      <button type="submit" class="btn btn-ghost">Erneut prüfen</button>
    </form>
  </div>
  %s
  <table>
    <thead>
      <tr>
        <th style="width:220px">Prüfung</th>
        <th style="width:110px">Status</th>
        <th>Detail / Hinweis</th>
      </tr>
    </thead>
    <tbody>%s</tbody>
  </table>
</div>`, summaryHTML, rows.String())

	return layout("Systemprüfung", body, "")
}

// ─── Postfix-Statusseite ──────────────────────────────────────────────────────

type PostfixPageData struct {
	QueueSize     int
	QueueList     string
	ServiceStatus string
	Flash         *Flash
}

func postfixPage(d PostfixPageData) string {
	queueClass := "badge-ok"
	queueLabel := "Leer"
	switch {
	case d.QueueSize < 0:
		queueClass = "badge-warn"
		queueLabel = "Unbekannt"
	case d.QueueSize == 0:
		queueClass = "badge-ok"
		queueLabel = "Leer"
	case d.QueueSize < 10:
		queueClass = "badge-warn"
		queueLabel = fmt.Sprintf("%d Mail(s)", d.QueueSize)
	default:
		queueClass = "badge-ko"
		queueLabel = fmt.Sprintf("%d Mails", d.QueueSize)
	}

	body := fmt.Sprintf(`
<div class="card">
  <div class="toolbar">
    <h2>Postfix-Status &amp; Warteschlange</h2>
    <form method="GET" action="/postfix">
      <button type="submit" class="btn btn-ghost">Aktualisieren</button>
    </form>
  </div>

  <div style="display:flex;gap:32px;margin-bottom:20px;flex-wrap:wrap;align-items:center">
    <div>
      <div style="font-size:.75rem;font-weight:600;color:#888;margin-bottom:4px;text-transform:uppercase;letter-spacing:.05em">Warteschlange</div>
      <span class="badge %s" style="font-size:.9rem;padding:5px 14px">%s</span>
    </div>
    <div style="display:flex;gap:8px;flex-wrap:wrap;margin-left:auto">
      <form method="POST" action="/postfix" style="display:inline">
        <input type="hidden" name="action" value="flush">
        <button type="submit" class="btn btn-primary" title="Sofortige Zustellung aller Mails versuchen">Warteschlange leeren (flush)</button>
      </form>
      <form method="POST" action="/postfix" style="display:inline">
        <input type="hidden" name="action" value="requeue">
        <button type="submit" class="btn btn-warn" title="Zurückgestellte Mails erneut einreihen">Zurückgestellte neu einreihen</button>
      </form>
      <form method="POST" action="/postfix" style="display:inline" onsubmit="return confirm('Postfix wirklich neu starten?')">
        <input type="hidden" name="action" value="restart">
        <button type="submit" class="btn btn-danger">Postfix neu starten</button>
      </form>
    </div>
  </div>
</div>

<div class="card">
  <h2>Warteschlangen-Inhalt</h2>
  <pre class="status-pre">%s</pre>
</div>

<div class="card">
  <h2>Dienst-Status</h2>
  <pre class="status-pre">%s</pre>
</div>`,
		queueClass, queueLabel,
		esc(d.QueueList),
		esc(d.ServiceStatus),
	)

	return layout("Postfix-Status", body, flashToHTML(d.Flash))
}
