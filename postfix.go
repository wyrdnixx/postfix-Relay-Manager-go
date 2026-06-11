package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Synthetische Hostnamen für Failover-Auflösung via /etc/hosts.
// Postfix probiert alle A-Records des Hostnamens der Reihe nach –
// fällt Server 1 aus, wird automatisch Server 2 verwendet.
const (
	relayIntHost  = "relay-int.prm"
	relayExtHost  = "relay-ext.prm"
	hostsBegin    = "# --- postfix-relay-manager begin ---"
	hostsEnd      = "# --- postfix-relay-manager end ---"
	hostsFile     = "/etc/hosts"
)

var hostedSectionRe = regexp.MustCompile(`(?s)` + hostsBegin + `.*?` + hostsEnd + `\n?`)

// manageHostsEntries schreibt die Relay-Server-IPs als A-Records in /etc/hosts
// und in den Postfix-Chroot (/var/spool/postfix/etc/hosts), damit der SMTP-Client
// von Postfix die synthetischen Hostnamen auflösen kann.
func manageHostsEntries() error {
	var section strings.Builder
	fmt.Fprintln(&section, hostsBegin)
	for _, srv := range relayServersInternal {
		fmt.Fprintf(&section, "%s\t%s\n", srv.Host, relayIntHost)
	}
	for _, srv := range relayServersExternal {
		fmt.Fprintf(&section, "%s\t%s\n", srv.Host, relayExtHost)
	}
	fmt.Fprintln(&section, hostsEnd)

	for _, path := range []string{hostsFile, "/var/spool/postfix/etc/hosts"} {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("hosts lesen (%s): %w", path, err)
		}
		content := strings.TrimRight(hostedSectionRe.ReplaceAllString(string(b), ""), "\n") + "\n"
		if err := os.WriteFile(path, []byte(content+section.String()), 0644); err != nil {
			return fmt.Errorf("hosts schreiben (%s): %w", path, err)
		}
	}
	return nil
}

// buildAllowedClients erzeugt den Inhalt der allowed_clients-Datei.
// Jeder registrierte Client bekommt einen FILTER-Eintrag, der auf den
// synthetischen Hostnamen zeigt – Postfix erledigt das Failover via /etc/hosts.
func buildAllowedClients(systems []System) string {
	var sb strings.Builder
	for _, s := range systems {
		host := relayIntHost
		port := 25
		if len(relayServersInternal) > 0 {
			port = relayServersInternal[0].Port
		}
		if s.Type == "external" {
			host = relayExtHost
			if len(relayServersExternal) > 0 {
				port = relayServersExternal[0].Port
			}
		}
		fmt.Fprintf(&sb, "%s FILTER smtp:[%s]:%d\n", s.IP, host, port)
	}
	return sb.String()
}

var mynetworksRe = regexp.MustCompile(`(?m)^mynetworks\s*=\s*(.*)$`)
var relayhostRe = regexp.MustCompile(`(?m)^relayhost\s*=\s*(.*)$`)
var inetInterfacesRe = regexp.MustCompile(`(?m)^inet_interfaces\s*=\s*(.*)$`)
var inetProtocolsRe = regexp.MustCompile(`(?m)^inet_protocols\s*=\s*(.*)$`)
var smtpHostLookupRe = regexp.MustCompile(`(?m)^smtp_host_lookup\s*=\s*(.*)$`)

// buildRelayhost gibt den Relayhost-Wert zurück.
// Durch den synthetischen Hostnamen probiert Postfix alle A-Records aus /etc/hosts.
func buildRelayhost() string {
	if len(relayServersInternal) == 0 {
		return ""
	}
	return fmt.Sprintf("[%s]:%d", relayIntHost, relayServersInternal[0].Port)
}

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

	mynetworksLine := "mynetworks = " + computeMynetworks(appData.Systems, appData.BaseMynetworks, appData.AllManagedIPs)
	relayhostLine := "relayhost = " + buildRelayhost()

	b, err := os.ReadFile(mainCfFile)
	if err != nil {
		return fmt.Errorf("main.cf lesen: %w", err)
	}
	content := string(b)

	if mynetworksRe.MatchString(content) {
		content = mynetworksRe.ReplaceAllString(content, mynetworksLine)
	} else {
		content += "\n" + mynetworksLine + "\n"
	}

	if relayhostRe.MatchString(content) {
		content = relayhostRe.ReplaceAllString(content, relayhostLine)
	} else {
		content += "\n" + relayhostLine + "\n"
	}

	// inet_interfaces auf "all" setzen falls noch auf loopback beschränkt
	if inetInterfacesRe.MatchString(content) {
		m := inetInterfacesRe.FindStringSubmatch(content)
		val := strings.TrimSpace(m[1])
		if val == "loopback-only" || val == "localhost" {
			content = inetInterfacesRe.ReplaceAllString(content, "inet_interfaces = all")
		}
	}

	// inet_protocols = ipv4: keine AAAA-Lookups für synthetische Hostnamen
	if inetProtocolsRe.MatchString(content) {
		content = inetProtocolsRe.ReplaceAllString(content, "inet_protocols = ipv4")
	} else {
		content += "\ninet_protocols = ipv4\n"
	}

	// smtp_host_lookup = native: OS-Resolver verwenden (liest /etc/hosts via nsswitch),
	// damit die synthetischen Hostnamen relay-int.prm / relay-ext.prm aufgelöst werden.
	if smtpHostLookupRe.MatchString(content) {
		content = smtpHostLookupRe.ReplaceAllString(content, "smtp_host_lookup = native")
	} else {
		content += "\nsmtp_host_lookup = native\n"
	}

	return os.WriteFile(mainCfFile, []byte(content), 0644)
}

// applyConfig schreibt die Postfix-Konfiguration und führt postmap + reload aus.
// Muss mit appMu gehalten aufgerufen werden.
func applyConfig() error {
	// /etc/hosts mit synthetischen Relay-Hostnamen für Failover aktualisieren
	if err := manageHostsEntries(); err != nil {
		return fmt.Errorf("hosts schreiben: %w", err)
	}

	// allowed_clients schreiben
	if err := os.WriteFile(allowedClientsFile, []byte(buildAllowedClients(appData.Systems)), 0644); err != nil {
		return fmt.Errorf("allowed_clients schreiben: %w", err)
	}

	// main.cf aktualisieren (setzt ggf. BaseMynetworks)
	if err := writeMainCf(); err != nil {
		return fmt.Errorf("main.cf schreiben: %w", err)
	}

	// postmap und Postfix-Reload (ohne sh -c, damit allowedClientsFile nicht als Shell-Code interpretiert werden kann)
	if out, err := runPrivileged("postmap", allowedClientsFile); err != nil {
		return fmt.Errorf("postmap fehlgeschlagen: %w\n%s", err, out)
	}
	if out, err := runPrivileged("postfix", "reload"); err != nil {
		return fmt.Errorf("reload fehlgeschlagen: %w\n%s", err, out)
	}
	return nil
}

// ─── Queue-Hilfsfunktionen ────────────────────────────────────────────────────

var queueCountRe = regexp.MustCompile(`(\d+) Requests?\.`)

// postfixQueueSize gibt die aktuelle Anzahl der Mails in der Warteschlange zurück.
// Gibt -1 zurück wenn der Aufruf fehlschlägt.
func postfixQueueSize() int {
	out, err := exec.Command("postqueue", "-p").Output()
	if err != nil {
		return -1
	}
	s := string(out)
	if strings.Contains(s, "Mail queue is empty") {
		return 0
	}
	if m := queueCountRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return -1
}

// postfixQueueList gibt die formatierte Warteschlange (postqueue -p) zurück.
func postfixQueueList() string {
	out, _ := exec.Command("postqueue", "-p").CombinedOutput()
	return strings.TrimSpace(string(out))
}

// postfixServiceStatus gibt den aktuellen systemctl-Status von Postfix zurück.
func postfixServiceStatus() string {
	out, _ := exec.Command("systemctl", "status", "postfix", "--no-pager", "-l", "--lines=20").CombinedOutput()
	return strings.TrimSpace(string(out))
}

// runPrivileged führt einen Befehl aus, mit sudo wenn der Prozess nicht als root läuft.
func runPrivileged(args ...string) (string, error) {
	if os.Getuid() != 0 {
		args = append([]string{"sudo"}, args...)
	}
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// postfixResend setzt zurückgestellte Mails zurück und triggert sofortige Zustellung.
func postfixResend() error {
	// Deferred → Incoming zurücksetzen
	if out, err := runPrivileged("postsuper", "-r", "ALL"); err != nil {
		return fmt.Errorf("postsuper -r ALL: %w\n%s", err, out)
	}
	// Sofortige Zustellung aller aktiven Mails anstoßen
	if out, err := runPrivileged("postqueue", "-f"); err != nil {
		return fmt.Errorf("postqueue -f: %w\n%s", err, out)
	}
	return nil
}

// postfixPurge löscht alle Mails aus der Warteschlange.
func postfixPurge() error {
	out, err := runPrivileged("postsuper", "-d", "ALL")
	if err != nil {
		return fmt.Errorf("postsuper -d ALL: %w\n%s", err, out)
	}
	return nil
}

// postfixSendTestMail sendet eine Test-Mail über den lokalen Postfix.
func postfixSendTestMail(to string) error {
	if strings.ContainsAny(to, "\r\n") || !strings.Contains(to, "@") {
		return fmt.Errorf("ungültige Empfängeradresse")
	}
	msg := fmt.Sprintf("From: relay-manager@localhost\r\nTo: %s\r\nSubject: Postfix Relay Manager – Test-Mail\r\n\r\nDiese Test-Mail wurde vom Postfix Relay Manager gesendet.\r\n", to)
	cmd := exec.Command("sendmail", "-t")
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sendmail: %w\n%s", err, string(out))
	}
	return nil
}

// postfixRestart startet den Postfix-Dienst neu.
func postfixRestart() error {
	out, err := runPrivileged("systemctl", "restart", "postfix")
	if err != nil {
		return fmt.Errorf("systemctl restart postfix: %w\n%s", err, out)
	}
	return nil
}
