package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// buildAllowedClients erzeugt den Inhalt der allowed_clients-Datei.
func buildAllowedClients(systems []System) string {
	var sb strings.Builder
	for _, s := range systems {
		servers := relayServersInternal
		if s.Type == "external" {
			servers = relayServersExternal
		}
		var parts []string
		for _, srv := range servers {
			parts = append(parts, fmt.Sprintf("smtp:[%s]:%d", srv.Host, srv.Port))
		}
		fmt.Fprintf(&sb, "%s FILTER %s\n", s.IP, strings.Join(parts, ","))
	}
	return sb.String()
}

var mynetworksRe = regexp.MustCompile(`(?m)^mynetworks\s*=\s*(.*)$`)

// computeMynetworks berechnet den neuen mynetworks-Wert ohne Seiteneffekte.
// Wird sowohl von writeMainCf als auch von der Vorschau-API verwendet.
func computeMynetworks(systems []System, baseMynetworks string, allManagedIPs []string) string {
	managedSet := make(map[string]bool)
	for _, ip := range allManagedIPs {
		managedSet[ip] = true
	}
	for _, s := range systems {
		managedSet[s.IP] = true
	}
	baseParts := strings.Fields(baseMynetworks)
	var filtered []string
	for _, p := range baseParts {
		if !managedSet[p] {
			filtered = append(filtered, p)
		}
	}
	for _, s := range systems {
		filtered = append(filtered, s.IP)
	}
	return strings.Join(filtered, " ")
}

// readMynetworksLine liest den aktuellen mynetworks-Wert aus main.cf.
func readMynetworksLine() (string, error) {
	b, err := os.ReadFile(mainCfFile)
	if err != nil {
		return "", err
	}
	m := mynetworksRe.FindSubmatch(b)
	if m == nil {
		return "", nil
	}
	return strings.TrimSpace(string(m[1])), nil
}

// writeMainCf schreibt die mynetworks-Zeile in main.cf neu.
// Muss mit appMu gehalten aufgerufen werden (liest/schreibt appData).
func writeMainCf() error {
	// Beim ersten Aufruf: Original-mynetworks sichern
	if appData.BaseMynetworks == "" {
		base, err := readMynetworksLine()
		if err != nil {
			return fmt.Errorf("mynetworks lesen: %w", err)
		}
		appData.BaseMynetworks = base
	}

	newLine := "mynetworks = " + computeMynetworks(appData.Systems, appData.BaseMynetworks, appData.AllManagedIPs)

	b, err := os.ReadFile(mainCfFile)
	if err != nil {
		return fmt.Errorf("main.cf lesen: %w", err)
	}
	content := string(b)
	if mynetworksRe.MatchString(content) {
		content = mynetworksRe.ReplaceAllString(content, newLine)
	} else {
		content += "\n" + newLine + "\n"
	}
	return os.WriteFile(mainCfFile, []byte(content), 0644)
}

// applyConfig schreibt die Postfix-Konfiguration und führt postmap + reload aus.
// Muss mit appMu gehalten aufgerufen werden.
func applyConfig() error {
	// allowed_clients schreiben
	if err := os.WriteFile(allowedClientsFile, []byte(buildAllowedClients(appData.Systems)), 0644); err != nil {
		return fmt.Errorf("allowed_clients schreiben: %w", err)
	}

	// main.cf aktualisieren (setzt ggf. BaseMynetworks)
	if err := writeMainCf(); err != nil {
		return fmt.Errorf("main.cf schreiben: %w", err)
	}

	// postmap und Postfix-Reload
	sudo := ""
	if os.Getuid() != 0 {
		sudo = "sudo "
	}
	cmdStr := fmt.Sprintf("%spostmap %s && %ssystemctl reload postfix", sudo, allowedClientsFile, sudo)
	out, err := exec.Command("sh", "-c", cmdStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("postmap/reload fehlgeschlagen: %w\n%s", err, string(out))
	}
	return nil
}
