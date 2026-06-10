package main

import (
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Hostname-Auflösung ──────────────────────────────────────────────────────

var (
	hostCache    sync.Map // ip -> string
	hostCacheAt  sync.Map // ip -> time.Time
	hostCacheTTL = 5 * time.Minute
)

// resolveHostname löst eine IP-Adresse per Reverse-DNS auf (gecacht, 5 min).
// CIDR-Ranges werden übersprungen.
func resolveHostname(ip string) string {
	if strings.Contains(ip, "/") {
		return ""
	}
	if v, ok := hostCache.Load(ip); ok {
		if t, ok := hostCacheAt.Load(ip); ok && time.Since(t.(time.Time)) < hostCacheTTL {
			return v.(string)
		}
	}
	names, err := net.LookupAddr(ip)
	hostname := ""
	if err == nil && len(names) > 0 {
		hostname = strings.TrimSuffix(names[0], ".")
	}
	hostCache.Store(ip, hostname)
	hostCacheAt.Store(ip, time.Now())
	return hostname
}

// ─── Health-Checks ───────────────────────────────────────────────────────────

// HealthResult beschreibt die TCP-Erreichbarkeit eines Relay-Server-Ports.
type HealthResult struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
}

func checkPort(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkRelayHealth prüft alle konfigurierten Relay-Server parallel.
func checkRelayHealth() []HealthResult {
	type task struct {
		server string
		port   int
		label  string
	}

	intSrvs, extSrvs := copyRelayServers()
	seen := make(map[string]bool)
	var tasks []task
	for _, srv := range intSrvs {
		key := net.JoinHostPort(srv.Host, strconv.Itoa(srv.Port))
		if !seen[key] {
			seen[key] = true
			tasks = append(tasks, task{srv.Host, srv.Port, "Intern"})
		}
	}
	for _, srv := range extSrvs {
		key := net.JoinHostPort(srv.Host, strconv.Itoa(srv.Port))
		if !seen[key] {
			seen[key] = true
			tasks = append(tasks, task{srv.Host, srv.Port, "Extern"})
		}
	}

	results := make([]HealthResult, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, tk task) {
			defer wg.Done()
			results[idx] = HealthResult{
				Server: tk.server,
				Port:   tk.port,
				Label:  tk.label,
				OK:     checkPort(tk.server, tk.port, 3*time.Second),
			}
		}(i, t)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Server != results[j].Server {
			return results[i].Server < results[j].Server
		}
		return results[i].Port < results[j].Port
	})
	return results
}
