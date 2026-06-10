package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckResult beschreibt das Ergebnis einer einzelnen Systemprüfung.
type CheckResult struct {
	Name    string
	Status  string // "ok", "warn", "err"
	Message string
	Detail  string // optionaler Hinweis zur Behebung
}

// runPostfixChecks führt alle Systemprüfungen aus und gibt die Ergebnisse zurück.
func runPostfixChecks() []CheckResult {
	var results []CheckResult

	// 1. Postfix installiert?
	if path, err := exec.LookPath("postfix"); err != nil {
		results = append(results, CheckResult{
			Name:    "Postfix installiert",
			Status:  "err",
			Message: "postfix-Binary nicht gefunden",
			Detail:  "Postfix scheint nicht installiert zu sein. Installieren Sie Postfix (z.B. apt install postfix).",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Postfix installiert",
			Status:  "ok",
			Message: path,
		})
	}

	// 2. Postfix-Dienst aktiv?
	out, err := exec.Command("systemctl", "is-active", "postfix").Output()
	state := strings.TrimSpace(string(out))
	if err != nil || state != "active" {
		status := state
		if status == "" {
			status = "unbekannt"
		}
		results = append(results, CheckResult{
			Name:    "Postfix-Dienst aktiv",
			Status:  "err",
			Message: fmt.Sprintf("Dienststatus: %s", status),
			Detail:  "Postfix ist nicht aktiv. Starten Sie den Dienst: systemctl start postfix",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Postfix-Dienst aktiv",
			Status:  "ok",
			Message: "aktiv (running)",
		})
	}

	// 3. postmap verfügbar?
	if path, err := exec.LookPath("postmap"); err != nil {
		results = append(results, CheckResult{
			Name:    "postmap verfügbar",
			Status:  "err",
			Message: "postmap-Binary nicht gefunden",
			Detail:  "postmap ist für das Erstellen der allowed_clients-Datenbank erforderlich. Es ist Teil des Postfix-Pakets.",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "postmap verfügbar",
			Status:  "ok",
			Message: path,
		})
	}

	// 4. main.cf lesbar?
	mainCfReadable := false
	var mainCfContent []byte
	if info, err := os.Stat(mainCfFile); err != nil {
		results = append(results, CheckResult{
			Name:    "main.cf lesbar",
			Status:  "err",
			Message: fmt.Sprintf("%s nicht gefunden", mainCfFile),
			Detail:  err.Error(),
		})
	} else if info.IsDir() {
		results = append(results, CheckResult{
			Name:    "main.cf lesbar",
			Status:  "err",
			Message: fmt.Sprintf("%s ist ein Verzeichnis", mainCfFile),
		})
	} else if mainCfContent, err = os.ReadFile(mainCfFile); err != nil {
		results = append(results, CheckResult{
			Name:    "main.cf lesbar",
			Status:  "err",
			Message: fmt.Sprintf("%s nicht lesbar", mainCfFile),
			Detail:  err.Error(),
		})
	} else {
		mainCfReadable = true
		results = append(results, CheckResult{
			Name:    "main.cf lesbar",
			Status:  "ok",
			Message: mainCfFile,
		})
	}

	// 5. Schreibrechte auf /etc/postfix/?
	postfixDir := filepath.Dir(allowedClientsFile)
	if _, err := os.Stat(postfixDir); err != nil {
		results = append(results, CheckResult{
			Name:    "Schreibrechte /etc/postfix/",
			Status:  "err",
			Message: fmt.Sprintf("Verzeichnis %s nicht vorhanden", postfixDir),
			Detail:  err.Error(),
		})
	} else {
		tmpFile := filepath.Join(postfixDir, ".relay-manager-writecheck")
		if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
			results = append(results, CheckResult{
				Name:    "Schreibrechte /etc/postfix/",
				Status:  "err",
				Message: fmt.Sprintf("Keine Schreibrechte auf %s", postfixDir),
				Detail:  "Der Prozess benötigt Schreibrechte, um allowed_clients und main.cf zu bearbeiten. Starten Sie den Dienst als root oder konfigurieren Sie sudo.",
			})
		} else {
			os.Remove(tmpFile)
			results = append(results, CheckResult{
				Name:    "Schreibrechte /etc/postfix/",
				Status:  "ok",
				Message: postfixDir,
			})
		}
	}

	// 6. mynetworks in main.cf vorhanden?
	if mainCfReadable {
		if !mynetworksRe.Match(mainCfContent) {
			results = append(results, CheckResult{
				Name:    "mynetworks in main.cf",
				Status:  "warn",
				Message: "mynetworks-Direktive nicht gefunden",
				Detail:  "Die Direktive 'mynetworks' fehlt in main.cf. Der Relay Manager wird sie beim ersten Speichern eines Systems automatisch anlegen.",
			})
		} else {
			m := mynetworksRe.FindSubmatch(mainCfContent)
			val := strings.TrimSpace(string(m[1]))
			results = append(results, CheckResult{
				Name:    "mynetworks in main.cf",
				Status:  "ok",
				Message: val,
			})
		}
	}

	// 7. allowed_clients in main.cf referenziert?
	if mainCfReadable {
		content := string(mainCfContent)
		if !strings.Contains(content, allowedClientsFile) && !strings.Contains(content, filepath.Base(allowedClientsFile)) {
			results = append(results, CheckResult{
				Name:    "allowed_clients referenziert",
				Status:  "err",
				Message: fmt.Sprintf("%s nicht in main.cf gefunden", allowedClientsFile),
				Detail: fmt.Sprintf(
					"Fügen Sie in %s folgende Zeile in smtpd_recipient_restrictions oder smtpd_relay_restrictions ein:\n  check_client_access hash:%s",
					mainCfFile, allowedClientsFile),
			})
		} else {
			results = append(results, CheckResult{
				Name:    "allowed_clients referenziert",
				Status:  "ok",
				Message: fmt.Sprintf("%s ist in main.cf referenziert", allowedClientsFile),
			})
		}
	}

	// 8. Rechte für systemctl reload postfix?
	if os.Getuid() == 0 {
		results = append(results, CheckResult{
			Name:    "Rechte für systemctl reload",
			Status:  "ok",
			Message: "Läuft als root",
		})
	} else {
		out, err := exec.Command("sudo", "-n", "systemctl", "is-active", "postfix").CombinedOutput()
		_ = out
		if err != nil {
			results = append(results, CheckResult{
				Name:    "Rechte für systemctl reload",
				Status:  "warn",
				Message: "sudo ohne Passwort nicht verfügbar",
				Detail: "Der Prozess läuft nicht als root und kann sudo nicht passwortlos ausführen.\n" +
					"Fügen Sie in /etc/sudoers (per visudo) folgende Zeile für den Dienstbenutzer hinzu:\n" +
					"  relay-manager ALL=(ALL) NOPASSWD: /usr/bin/postmap, /usr/bin/systemctl reload postfix",
			})
		} else {
			results = append(results, CheckResult{
				Name:    "Rechte für systemctl reload",
				Status:  "ok",
				Message: "sudo ohne Passwort konfiguriert",
			})
		}
	}

	return results
}
