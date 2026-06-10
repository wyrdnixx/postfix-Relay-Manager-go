package main

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogEntry repräsentiert einen abgelehnten Relay-Versuch.
type LogEntry struct {
	IP        string    `json:"ip"`
	Recipient string    `json:"recipient"`
	Time      time.Time `json:"-"`
	TimeStr   string    `json:"timeStr"`
	AddURL    string    `json:"addUrl"`
}

var (
	logCache    []LogEntry
	logCacheAt  time.Time
	logCacheMu  sync.Mutex
	logCacheTTL = 60 * time.Second
)

// Regex: IP aus [x.x.x.x] und Empfänger vor ": Relay access denied"
var denyRe = regexp.MustCompile(
	`NOQUEUE: reject: RCPT from \S+\[(\d{1,3}(?:\.\d{1,3}){3})\]: \S+ \S+ ([^:]+): Relay access denied`,
)

// Timestamp-Formate
var (
	isoTimeRe = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})`)
	classicRe = regexp.MustCompile(`^(\w{3}) {1,2}(\d{1,2}) (\d{2}):(\d{2}):(\d{2})`)
)

var monthMap = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

// parseLogTime erkennt ISO-8601- und klassische Syslog-Zeitstempel.
func parseLogTime(line string, year int) time.Time {
	// Format 1: ISO 8601 – z.B. "2026-03-06T10:23:45"
	if m := isoTimeRe.FindStringSubmatch(line); m != nil {
		t, err := time.ParseInLocation("2006-01-02T15:04:05",
			m[1]+"-"+m[2]+"-"+m[3]+"T"+m[4]+":"+m[5]+":"+m[6], time.Local)
		if err == nil {
			return t
		}
	}
	// Format 2: Klassisches Syslog – z.B. "Mar  6 10:23:45"
	if m := classicRe.FindStringSubmatch(line); m != nil {
		mon, ok := monthMap[m[1]]
		if ok {
			return time.Date(year, mon, atoiSimple(m[2]),
				atoiSimple(m[3]), atoiSimple(m[4]), atoiSimple(m[5]), 0, time.Local)
		}
	}
	return time.Time{}
}

func atoiSimple(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	return t.Format("02.01.2006 15:04")
}

// getDeniedLog liest und parst Mail-Log-Einträge, gecacht für 60 Sekunden.
func getDeniedLog() []LogEntry {
	logCacheMu.Lock()
	defer logCacheMu.Unlock()

	if time.Since(logCacheAt) < logCacheTTL && logCache != nil {
		return logCache
	}

	lines := readLogLines()
	year := time.Now().Year()
	var entries []LogEntry

	for _, line := range lines {
		if !strings.Contains(line, "Relay access denied") {
			continue
		}
		m := denyRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := parseLogTime(line, year)
		entries = append(entries, LogEntry{
			IP:        m[1],
			Recipient: m[2],
			Time:      t,
			TimeStr:   fmtTime(t),
			AddURL:    "/add?ip=" + m[1],
		})
	}

	// Neueste zuerst, maximal 200 Einträge
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.After(entries[j].Time)
	})
	if len(entries) > 200 {
		entries = entries[:200]
	}

	logCache = entries
	logCacheAt = time.Now()
	return entries
}

// readLogLines liest mailLogFile oder fällt auf journalctl zurück.
func readLogLines() []string {
	f, err := os.Open(mailLogFile)
	if err == nil {
		defer f.Close()
		var lines []string
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if len(lines) > 100000 {
			lines = lines[len(lines)-100000:]
		}
		return lines
	}

	out, err := exec.Command("journalctl", "-u", "postfix", "--no-pager", "-n", "100000").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > 100000 {
		lines = lines[len(lines)-100000:]
	}
	return lines
}
