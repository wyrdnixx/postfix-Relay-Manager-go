package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Zeitstempel aus log entfernen – systemd/journald stempelt selbst
	log.SetFlags(0)
	log.SetPrefix("postfix-relay-manager: ")

	// Datenpfad relativ zur ausführbaren Datei bestimmen
	exe, err := os.Executable()
	if err != nil {
		log.Fatal("Ausführungspfad nicht ermittelbar: ", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		log.Fatal("Symlink-Auflösung fehlgeschlagen: ", err)
	}
	dataFilePath = filepath.Join(filepath.Dir(exe), "data.json")
	log.Printf("Datenpfad: %s", dataFilePath)

	// Zertifikat und Schlüssel ebenfalls relativ zur Exe
	certFile := filepath.Join(filepath.Dir(exe), "cert.pem")
	keyFile := filepath.Join(filepath.Dir(exe), "key.pem")

	if err := loadData(); err != nil {
		log.Fatal("data.json laden fehlgeschlagen: ", err)
	}

	if err := validateConfig(); err != nil {
		log.Fatal("Konfigurationsfehler in data.json: ", err)
	}

	setPasswordHash(appData.Config.AdminPasswordHash)

	// Nutzungsstatistik: Mail-Log periodisch im Hintergrund auswerten
	go startUsageStatsUpdater()

	log.Printf("Starte auf https://0.0.0.0%s", listenAddr)

	mux := http.NewServeMux()

	// Öffentliche Routen
	mux.HandleFunc("GET /login", handleLoginGet)
	mux.HandleFunc("POST /login", handleLoginPost)
	mux.HandleFunc("GET /logout", handleLogout)

	// Geschützte Routen
	mux.HandleFunc("GET /{$}", authMiddleware(handleIndex))
	mux.HandleFunc("GET /add", authMiddleware(handleAddGet))
	mux.HandleFunc("POST /add", authMiddleware(handleAddPost))
	mux.HandleFunc("GET /edit/{id}", authMiddleware(handleEditGet))
	mux.HandleFunc("POST /edit/{id}", authMiddleware(handleEditPost))
	mux.HandleFunc("POST /delete/{id}", authMiddleware(handleDeletePost))
	mux.HandleFunc("GET /logs", authMiddleware(handleLogsPage))
	mux.HandleFunc("GET /settings", authMiddleware(handleSettingsGet))
	mux.HandleFunc("POST /settings", authMiddleware(handleSettingsPost))
	mux.HandleFunc("POST /settings/relay", authMiddleware(handleSettingsRelayPost))
	mux.HandleFunc("GET /bulk-add", authMiddleware(handleBulkAddGet))
	mux.HandleFunc("GET /syscheck", authMiddleware(handleSysCheckGet))
	mux.HandleFunc("GET /postfix", authMiddleware(handlePostfixGet))
	mux.HandleFunc("POST /postfix", authMiddleware(handlePostfixPost))
	mux.HandleFunc("POST /bulk-add", authMiddleware(handleBulkAddPost))

	// APIs
	mux.HandleFunc("GET /api/health", authMiddleware(handleApiHealth))
	mux.HandleFunc("GET /api/logs", authMiddleware(handleApiLogs))
	mux.HandleFunc("GET /api/maillog", authMiddleware(handleApiMailLog))
	mux.HandleFunc("POST /api/preview", authMiddleware(handleApiPreview))
	mux.HandleFunc("GET /api/resolve", authMiddleware(handleApiResolve))

	// Nur TLS 1.2 + 1.3, moderne AEAD-Cipher mit Forward Secrecy.
	// Die CipherSuites-Liste gilt ausschließlich für TLS 1.2; die
	// TLS-1.3-Cipher legt Go selbst fest und sind allesamt sicher.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	srv := &http.Server{
		Addr:      listenAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	log.Fatal(srv.ListenAndServeTLS(certFile, keyFile))
}
