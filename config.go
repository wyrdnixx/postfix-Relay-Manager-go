package main

// ─── Konfiguration – Fallback-Werte (werden durch data.json überschrieben) ───
// Relay-Server und Admin-Zugangsdaten haben KEINE Fallbacks und müssen
// zwingend in data.json unter "config" definiert sein.

var (
	// HTTP-Port
	listenAddr = ":8080"

	// Postfix-Konfigurationsdateien
	allowedClientsFile = "/etc/postfix/allowed_clients"
	mainCfFile         = "/etc/postfix/main.cf"

	// Mail-Log
	mailLogFile = "/var/log/mail.log"

	// Relay-Server – keine Fallbacks, müssen in data.json gesetzt sein
	relayServersInternal []RelayServer
	relayServersExternal []RelayServer

	// Admin-Zugangsdaten – keine Fallbacks, müssen in data.json gesetzt sein
	adminUsername string
)
