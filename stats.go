package main

import (
	"log"
	"strings"
	"time"
)

// statsUpdateInterval bestimmt, wie oft das Mail-Log für die
// Nutzungsstatistik erneut eingelesen wird.
var statsUpdateInterval = 60 * time.Second

// LastUsedDisplay liefert den Zeitpunkt der letzten Nutzung als lesbaren
// String für die UI (oder "–", wenn das System noch nie gesendet hat).
func (s System) LastUsedDisplay() string {
	if s.LastUsed.IsZero() {
		return "–"
	}
	return s.LastUsed.Format("02.01.2006 15:04")
}

// startUsageStatsUpdater startet die periodische Log-Auswertung im
// Hintergrund. Wird einmal beim Programmstart aus main() aufgerufen
// (als eigene Goroutine: `go startUsageStatsUpdater()`).
func startUsageStatsUpdater() {
	updateUsageStats() // erste Auswertung sofort
	ticker := time.NewTicker(statsUpdateInterval)
	defer ticker.Stop()
	for range ticker.C {
		updateUsageStats()
	}
}

// updateUsageStats liest das Mail-Log, zählt erfolgreiche Zustellungen
// (status=sent) je Absender-IP und schreibt usageCount / lastUsed in die
// betroffenen Systeme zurück.
//
// Doppelzählung über mehrere Durchläufe hinweg wird über den High-Water-Mark
// appData.LastStatsTime verhindert: Es werden nur Einträge gezählt, die zeitlich
// nach der letzten Auswertung liegen. Beim allerersten Lauf (LastStatsTime ist
// leer) wird der aktuell im Log sichtbare Verlauf einmalig als Startwert erfasst.
func updateUsageStats() {
	// Log OHNE gehaltenen appMu-Lock einlesen und parsen – kann groß sein.
	lines := readLogLines()
	year := time.Now().Year()
	entries := parseSentEntries(lines, year)
	if len(entries) == 0 {
		return
	}

	appMu.Lock()
	defer appMu.Unlock()

	hw := appData.LastStatsTime
	firstRun := hw.IsZero()

	// Index: System-IP -> Position in appData.Systems
	idxByIP := make(map[string]int, len(appData.Systems))
	for i := range appData.Systems {
		idxByIP[appData.Systems[i].IP] = i
	}

	newest := hw
	changed := false

	for _, e := range entries {
		if e.Time.IsZero() {
			continue // nicht datierbare Zeile – für die Zählung ignorieren
		}
		// Nur Einträge zählen, die neuer als die letzte Auswertung sind.
		if !firstRun && !e.Time.After(hw) {
			continue
		}
		if e.Time.After(newest) {
			newest = e.Time
		}
		i, ok := idxByIP[e.ClientIP]
		if !ok {
			continue // Absender-IP gehört zu keinem verwalteten System
		}
		appData.Systems[i].UsageCount++
		if e.Time.After(appData.Systems[i].LastUsed) {
			appData.Systems[i].LastUsed = e.Time
		}
		changed = true
	}

	// High-Water-Mark immer voranstellen – auch wenn kein System betroffen war –,
	// damit dieselben Zeilen im nächsten Lauf nicht erneut betrachtet werden.
	if newest.After(appData.LastStatsTime) {
		appData.LastStatsTime = newest
		changed = true
	}

	if changed {
		if err := saveData(); err != nil {
			log.Printf("Nutzungsstatistik: Speichern fehlgeschlagen: %v", err)
		}
	}
}

// parseSentEntries extrahiert erfolgreiche Zustellungen (status=sent) mit
// zugeordneter Absender-IP aus den Log-Zeilen. Nutzt dieselben Regexes und
// Zeit-Parser wie logs.go, damit das Format-Verständnis an einer Stelle bleibt.
func parseSentEntries(lines []string, year int) []MailLogEntry {
	// Pass 1: QueueID -> Absender-IP (aus den smtpd-Zeilen)
	clientByID := make(map[string]string)
	for _, line := range lines {
		if m := smtpClientRe.FindStringSubmatch(line); m != nil {
			clientByID[m[1]] = m[2]
		}
	}

	// Pass 2: nur Zustellungen mit status=sent
	var out []MailLogEntry
	for _, line := range lines {
		if !strings.Contains(line, "status=sent") {
			continue
		}
		m := mailStatusRe.FindStringSubmatch(line)
		if m == nil || m[5] != "sent" {
			continue
		}
		out = append(out, MailLogEntry{
			Time:      parseLogTime(line, year),
			QueueID:   m[1],
			ClientIP:  clientByID[m[1]],
			Recipient: m[2],
			Relay:     m[3],
			Status:    m[5],
		})
	}
	return out
}
