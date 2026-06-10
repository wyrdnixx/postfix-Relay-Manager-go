package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Datenpfad relativ zur ausführbaren Datei bestimmen
	exe, err := os.Executable()
	if err != nil {
		log.Fatal("Fehler beim Bestimmen des Ausführungspfads:", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		log.Fatal("Fehler beim Auflösen von Symlinks:", err)
	}
	dataFilePath = filepath.Join(filepath.Dir(exe), "data.json")

	if err := loadData(); err != nil {
		log.Fatal("Fehler beim Laden von data.json:", err)
	}

	if err := validateConfig(); err != nil {
		log.Fatal("Konfigurationsfehler in data.json: ", err)
	}

	setPasswordHash(appData.Config.AdminPasswordHash)

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

	fmt.Printf("Postfix Relay Manager läuft auf http://0.0.0.0%s\n", listenAddr)
	fmt.Println("SHA-256 für neues Passwort generieren: echo -n 'NEUES_PASSWORT' | sha256sum")
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
