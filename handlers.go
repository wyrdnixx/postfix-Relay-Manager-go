package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ─── Passwort-Hash (thread-safe, zur Laufzeit änderbar) ──────────────────────

var (
	ipRegex        = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}(/\d{1,2})?$`)
	passwordHashMu sync.RWMutex
	passwordHash   string
)

var validCategories = map[string]bool{
	"printer": true, "server": true, "scanner": true,
	"network": true, "other": true, "": true,
}

func getPasswordHash() string {
	passwordHashMu.RLock()
	defer passwordHashMu.RUnlock()
	return passwordHash
}

func setPasswordHash(h string) {
	passwordHashMu.Lock()
	passwordHash = h
	passwordHashMu.Unlock()
}

func isValidIP(ip string) bool { return ipRegex.MatchString(ip) }

func writeHTML(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, content)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, sess := getSession(r)
		if sess == nil || !sess.Authenticated {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// ─── Auth ────────────────────────────────────────────────────────────────────

func handleLoginGet(w http.ResponseWriter, r *http.Request) {
	_, sess := getSession(r)
	if sess != nil && sess.Authenticated {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	writeHTML(w, loginPage(false))
}

func handleLoginPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := r.FormValue("username")
	pass := r.FormValue("password")
	h := sha256.Sum256([]byte(pass))
	if user == adminUsername && fmt.Sprintf("%x", h) == getPasswordHash() {
		_, sess := createSession(w)
		sess.Authenticated = true
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
	writeHTML(w, loginPage(true))
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	id, _ := getSession(r)
	if id != "" {
		deleteSession(w, id)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ─── Übersicht ───────────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	_, sess := getSession(r)
	var flash *Flash
	if sess != nil {
		flash = sess.getFlash()
	}
	appMu.Lock()
	systems := make([]System, len(appData.Systems))
	copy(systems, appData.Systems)
	appMu.Unlock()

	writeHTML(w, indexPage(systems, flash))
}

// ─── System hinzufügen ────────────────────────────────────────────────────────

func handleAddGet(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	writeHTML(w, systemFormPage(System{IP: ip}, "/add", "System hinzufügen", "Hinzufügen", ""))
}

func handleAddPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ip := strings.TrimSpace(r.FormValue("ip"))
	name := strings.TrimSpace(r.FormValue("name"))
	sysType := r.FormValue("type")
	category := r.FormValue("category")
	if !validCategories[category] {
		category = "other"
	}
	sys := System{IP: ip, Name: name, Type: sysType, Category: category}

	if !isValidIP(ip) || (sysType != "internal" && sysType != "external") {
		writeHTML(w, systemFormPage(sys, "/add", "System hinzufügen", "Hinzufügen",
			"Bitte eine gültige IP-Adresse und einen Versandtyp angeben."))
		return
	}

	appMu.Lock()
	for _, s := range appData.Systems {
		if s.IP == ip {
			appMu.Unlock()
			writeHTML(w, systemFormPage(sys, "/add", "System hinzufügen", "Hinzufügen",
				fmt.Sprintf("Die IP-Adresse %s ist bereits vorhanden.", html.EscapeString(ip))))
			return
		}
	}
	appData.Systems = append(appData.Systems, System{
		ID:       fmt.Sprintf("%d", time.Now().UnixMilli()),
		Name:     name,
		IP:       ip,
		Type:     sysType,
		Category: category,
	})
	addManagedIP(ip)
	err := applyConfig()
	_ = saveData()
	appMu.Unlock()

	_, sess := getSession(r)
	if sess != nil {
		if err != nil {
			sess.setFlash(&Flash{"System gespeichert, aber Postfix-Reload fehlgeschlagen: " + err.Error(), "err"})
		} else {
			sess.setFlash(&Flash{fmt.Sprintf("System %s erfolgreich hinzugefügt. Postfix wurde neu geladen.", ip), "ok"})
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// ─── System bearbeiten ────────────────────────────────────────────────────────

func handleEditGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	appMu.Lock()
	var found *System
	for i := range appData.Systems {
		if appData.Systems[i].ID == id {
			s := appData.Systems[i]
			found = &s
			break
		}
	}
	appMu.Unlock()
	if found == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	writeHTML(w, systemFormPage(*found, "/edit/"+id, "System bearbeiten", "Speichern", ""))
}

func handleEditPost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.ParseForm()
	ip := strings.TrimSpace(r.FormValue("ip"))
	name := strings.TrimSpace(r.FormValue("name"))
	sysType := r.FormValue("type")
	category := r.FormValue("category")
	if !validCategories[category] {
		category = "other"
	}
	sys := System{ID: id, IP: ip, Name: name, Type: sysType, Category: category}

	if !isValidIP(ip) || (sysType != "internal" && sysType != "external") {
		writeHTML(w, systemFormPage(sys, "/edit/"+id, "System bearbeiten", "Speichern",
			"Bitte eine gültige IP-Adresse und einen Versandtyp angeben."))
		return
	}

	appMu.Lock()
	idx := -1
	for i, s := range appData.Systems {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		appMu.Unlock()
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	for i, s := range appData.Systems {
		if i != idx && s.IP == ip {
			appMu.Unlock()
			writeHTML(w, systemFormPage(sys, "/edit/"+id, "System bearbeiten", "Speichern",
				fmt.Sprintf("Die IP-Adresse %s wird bereits von einem anderen System verwendet.", html.EscapeString(ip))))
			return
		}
	}
	appData.Systems[idx] = sys
	addManagedIP(ip)
	err := applyConfig()
	_ = saveData()
	appMu.Unlock()

	_, sess := getSession(r)
	if sess != nil {
		if err != nil {
			sess.setFlash(&Flash{"Gespeichert, aber Postfix-Reload fehlgeschlagen: " + err.Error(), "err"})
		} else {
			sess.setFlash(&Flash{fmt.Sprintf("System %s erfolgreich aktualisiert. Postfix wurde neu geladen.", ip), "ok"})
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// ─── System löschen ──────────────────────────────────────────────────────────

func handleDeletePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	appMu.Lock()
	idx := -1
	var deletedIP string
	for i, s := range appData.Systems {
		if s.ID == id {
			idx = i
			deletedIP = s.IP
			break
		}
	}
	if idx == -1 {
		appMu.Unlock()
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	appData.Systems = append(appData.Systems[:idx], appData.Systems[idx+1:]...)
	err := applyConfig()
	_ = saveData()
	appMu.Unlock()

	_, sess := getSession(r)
	if sess != nil {
		if err != nil {
			sess.setFlash(&Flash{"Gelöscht, aber Postfix-Reload fehlgeschlagen: " + err.Error(), "err"})
		} else {
			sess.setFlash(&Flash{fmt.Sprintf("System %s erfolgreich entfernt. Postfix wurde neu geladen.", deletedIP), "ok"})
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// ─── Protokoll ────────────────────────────────────────────────────────────────

func handleLogsPage(w http.ResponseWriter, r *http.Request) {
	appMu.Lock()
	knownIPs := make(map[string]bool, len(appData.Systems))
	for _, s := range appData.Systems {
		knownIPs[s.IP] = true
	}
	appMu.Unlock()

	all := getDeniedLog()
	var filtered []LogEntry
	for _, e := range all {
		if !knownIPs[e.IP] {
			filtered = append(filtered, e)
		}
	}
	writeHTML(w, logsPage(filtered))
}

// ─── Einstellungen ────────────────────────────────────────────────────────────

func handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	_, sess := getSession(r)
	var flash *Flash
	if sess != nil {
		flash = sess.getFlash()
	}
	appMu.Lock()
	intSrvs := append([]RelayServer{}, relayServersInternal...)
	extSrvs := append([]RelayServer{}, relayServersExternal...)
	appMu.Unlock()
	writeHTML(w, settingsPage(flash, intSrvs, extSrvs))
}

func handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	currentPw := r.FormValue("current_password")
	newPw := r.FormValue("new_password")
	confirmPw := r.FormValue("confirm_password")

	h := sha256.Sum256([]byte(currentPw))
	if fmt.Sprintf("%x", h) != getPasswordHash() {
		writeHTML(w, settingsPageCurrent(&Flash{"Das aktuelle Passwort ist falsch.", "err"}))
		return
	}
	if len(newPw) < 8 {
		writeHTML(w, settingsPageCurrent(&Flash{"Das neue Passwort muss mindestens 8 Zeichen haben.", "err"}))
		return
	}
	if newPw != confirmPw {
		writeHTML(w, settingsPageCurrent(&Flash{"Die neuen Passwörter stimmen nicht überein.", "err"}))
		return
	}

	newHash := fmt.Sprintf("%x", sha256.Sum256([]byte(newPw)))
	setPasswordHash(newHash)

	appMu.Lock()
	if appData.Config == nil {
		appData.Config = &AppConfig{}
	}
	appData.Config.AdminPasswordHash = newHash
	_ = saveData()
	appMu.Unlock()

	_, sess := getSession(r)
	if sess != nil {
		sess.setFlash(&Flash{"Passwort erfolgreich geändert.", "ok"})
	}
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsPageCurrent(flash *Flash) string {
	appMu.Lock()
	intSrvs := append([]RelayServer{}, relayServersInternal...)
	extSrvs := append([]RelayServer{}, relayServersExternal...)
	appMu.Unlock()
	return settingsPage(flash, intSrvs, extSrvs)
}

func handleSettingsRelayPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	parseServers := func(prefix string) ([]RelayServer, error) {
		hosts := r.Form[prefix+"_host"]
		ports := r.Form[prefix+"_port"]
		var out []RelayServer
		for i, h := range hosts {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			portStr := ""
			if i < len(ports) {
				portStr = strings.TrimSpace(ports[i])
			}
			p := 0
			if _, err := fmt.Sscanf(portStr, "%d", &p); err != nil || p < 1 || p > 65535 {
				return nil, fmt.Errorf("ungültiger Port für %s: %q", h, portStr)
			}
			out = append(out, RelayServer{Host: h, Port: p})
		}
		return out, nil
	}

	intSrvs, err := parseServers("int")
	if err != nil {
		writeHTML(w, settingsPageCurrent(&Flash{Msg: err.Error(), Type: "err"}))
		return
	}
	extSrvs, err := parseServers("ext")
	if err != nil {
		writeHTML(w, settingsPageCurrent(&Flash{Msg: err.Error(), Type: "err"}))
		return
	}
	if len(intSrvs) == 0 {
		writeHTML(w, settingsPageCurrent(&Flash{Msg: "Mindestens ein interner Relay-Server erforderlich.", Type: "err"}))
		return
	}
	if len(extSrvs) == 0 {
		writeHTML(w, settingsPageCurrent(&Flash{Msg: "Mindestens ein externer Relay-Server erforderlich.", Type: "err"}))
		return
	}

	appMu.Lock()
	relayServersInternal = intSrvs
	relayServersExternal = extSrvs
	if appData.Config == nil {
		appData.Config = &AppConfig{}
	}
	appData.Config.RelayServersInternal = intSrvs
	appData.Config.RelayServersExternal = extSrvs
	_ = saveData()
	applyErr := applyConfig()
	appMu.Unlock()

	if applyErr != nil {
		writeHTML(w, settingsPage(&Flash{Msg: "Gespeichert, aber Postfix-Reload fehlgeschlagen: " + applyErr.Error(), Type: "err"}, intSrvs, extSrvs))
		return
	}

	_, sess := getSession(r)
	if sess != nil {
		sess.setFlash(&Flash{Msg: "Relay-Server gespeichert und Postfix-Konfiguration angewendet.", Type: "ok"})
	}
	http.Redirect(w, r, "/settings", http.StatusFound)
}

// ─── APIs ─────────────────────────────────────────────────────────────────────

func handleApiHealth(w http.ResponseWriter, _ *http.Request) {
	results := checkRelayHealth()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func handleApiLogs(w http.ResponseWriter, _ *http.Request) {
	appMu.Lock()
	knownIPs := make(map[string]bool, len(appData.Systems))
	for _, s := range appData.Systems {
		knownIPs[s.IP] = true
	}
	appMu.Unlock()

	all := getDeniedLog()
	filtered := make([]LogEntry, 0)
	for _, e := range all {
		if !knownIPs[e.IP] {
			filtered = append(filtered, e)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// handleApiPreview berechnet, wie allowed_clients und mynetworks nach dem
// Speichern eines Systems aussehen würden – ohne etwas zu ändern.
func handleApiPreview(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ip := strings.TrimSpace(r.FormValue("ip"))
	sysType := r.FormValue("type")
	sysID := r.FormValue("sys_id")

	if ip == "" {
		http.Error(w, `{"error":"IP fehlt"}`, http.StatusBadRequest)
		return
	}

	appMu.Lock()
	// Hypothetische Systemliste aufbauen
	hypo := make([]System, len(appData.Systems))
	copy(hypo, appData.Systems)
	if sysID != "" {
		for i, s := range hypo {
			if s.ID == sysID {
				hypo[i].IP = ip
				hypo[i].Type = sysType
				break
			}
		}
	} else {
		hypo = append(hypo, System{IP: ip, Type: sysType})
	}

	hypoManaged := make([]string, len(appData.AllManagedIPs))
	copy(hypoManaged, appData.AllManagedIPs)
	found := false
	for _, m := range hypoManaged {
		if m == ip {
			found = true
			break
		}
	}
	if !found {
		hypoManaged = append(hypoManaged, ip)
	}
	base := appData.BaseMynetworks
	appMu.Unlock()

	// baseMynetworks aus main.cf lesen, falls noch nicht gesetzt
	if base == "" {
		if b, err := readMynetworksLine(); err == nil {
			base = b
		}
	}

	ac := buildAllowedClients(hypo)
	mn := "mynetworks = " + computeMynetworks(hypo, base, hypoManaged)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"allowedClients": ac,
		"mynetworks":     mn,
	})
}

// handleApiResolve löst alle aktuellen System-IPs per Reverse-DNS auf.
func handleApiResolve(w http.ResponseWriter, _ *http.Request) {
	appMu.Lock()
	ips := make([]string, 0, len(appData.Systems))
	for _, s := range appData.Systems {
		ips = append(ips, s.IP)
	}
	appMu.Unlock()

	result := make(map[string]string, len(ips))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			hn := resolveHostname(ip)
			if hn != "" {
				mu.Lock()
				result[ip] = hn
				mu.Unlock()
			}
		}(ip)
	}
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ─── Bulk-Import ──────────────────────────────────────────────────────────────

// BulkResult fasst die Ergebnisse eines Bulk-Imports zusammen.
type BulkResult struct {
	Added   []string // erfolgreich hinzugefügte IPs
	Skipped []string // bereits vorhanden
	Invalid []string // ungültige Eingaben
	ApplyErr error
}

func handleBulkAddGet(w http.ResponseWriter, r *http.Request) {
	writeHTML(w, bulkAddPage(""))
}

func handleBulkAddPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	raw := r.FormValue("ips")
	name := strings.TrimSpace(r.FormValue("name"))
	sysType := r.FormValue("type")
	category := r.FormValue("category")
	if !validCategories[category] {
		category = "other"
	}

	// IPs parsen: Zeilenumbrüche UND Kommas als Trenner, # = Kommentar
	var candidates []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				candidates = append(candidates, part)
			}
		}
	}

	if len(candidates) == 0 {
		writeHTML(w, bulkAddPage("Keine IPs in der Eingabe gefunden."))
		return
	}

	var invalid []string
	var valid []string
	seen := make(map[string]bool)
	for _, ip := range candidates {
		if !isValidIP(ip) {
			invalid = append(invalid, ip)
		} else if !seen[ip] {
			seen[ip] = true
			valid = append(valid, ip)
		}
	}

	appMu.Lock()
	existingIPs := make(map[string]bool, len(appData.Systems))
	for _, s := range appData.Systems {
		existingIPs[s.IP] = true
	}

	now := time.Now().UnixMilli()
	var added, skipped []string
	for i, ip := range valid {
		if existingIPs[ip] {
			skipped = append(skipped, ip)
			continue
		}
		appData.Systems = append(appData.Systems, System{
			ID:       fmt.Sprintf("%d", now+int64(i)),
			Name:     name,
			IP:       ip,
			Type:     sysType,
			Category: category,
		})
		addManagedIP(ip)
		existingIPs[ip] = true
		added = append(added, ip)
	}

	result := &BulkResult{Added: added, Skipped: skipped, Invalid: invalid}
	if len(added) > 0 {
		result.ApplyErr = applyConfig()
		_ = saveData()
	}
	appMu.Unlock()

	// Bei komplett sauberem Import direkt zur Übersicht
	if len(skipped) == 0 && len(invalid) == 0 && result.ApplyErr == nil {
		_, sess := getSession(r)
		if sess != nil {
			sess.setFlash(&Flash{
				fmt.Sprintf("%d IP(s) erfolgreich importiert. Postfix wurde neu geladen.", len(added)),
				"ok",
			})
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Ansonsten Detailseite zeigen
	writeHTML(w, bulkResultPage(result))
}

// ─── Systemprüfung ────────────────────────────────────────────────────────────

func handleSysCheckGet(w http.ResponseWriter, _ *http.Request) {
	results := runPostfixChecks()
	writeHTML(w, sysCheckPage(results))
}

// ─── Postfix-Statusseite ──────────────────────────────────────────────────────

func postfixPageData(flash *Flash) PostfixPageData {
	return PostfixPageData{
		QueueSize:     postfixQueueSize(),
		QueueList:     postfixQueueList(),
		ServiceStatus: postfixServiceStatus(),
		Flash:         flash,
	}
}

func handlePostfixGet(w http.ResponseWriter, _ *http.Request) {
	writeHTML(w, postfixPage(postfixPageData(nil)))
}

func handlePostfixPost(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	var err error
	var successMsg string

	switch action {
	case "testmail":
		to := strings.TrimSpace(r.FormValue("to"))
		if to == "" {
			flash := &Flash{Msg: "Bitte eine Empfänger-Adresse angeben.", Type: "err"}
			writeHTML(w, postfixPage(postfixPageData(flash)))
			return
		}
		err = postfixSendTestMail(to)
		successMsg = fmt.Sprintf("Test-Mail an %s wurde eingereiht.", html.EscapeString(to))
	case "resend":
		err = postfixResend()
		successMsg = "Mails wurden zurückgesetzt und zur sofortigen Zustellung freigegeben."
	case "purge":
		err = postfixPurge()
		successMsg = "Alle Mails in der Warteschlange wurden gelöscht."
	case "restart":
		appMu.Lock()
		err = applyConfig()
		appMu.Unlock()
		if err == nil {
			err = postfixRestart()
		}
		successMsg = "Konfiguration angewendet und Postfix neu gestartet."
	default:
		http.Error(w, "Unbekannte Aktion", http.StatusBadRequest)
		return
	}

	var flash *Flash
	if err != nil {
		flash = &Flash{Msg: "Fehler: " + err.Error(), Type: "err"}
	} else {
		flash = &Flash{Msg: successMsg, Type: "ok"}
	}
	writeHTML(w, postfixPage(postfixPageData(flash)))
}
