package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// RelayServer beschreibt einen SMTP-Relay-Server mit Host und Port.
type RelayServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// AppConfig enthält die in data.json persistierte Laufzeitkonfiguration.
type AppConfig struct {
	RelayServersInternal []RelayServer `json:"relayServersInternal,omitempty"`
	RelayServersExternal []RelayServer `json:"relayServersExternal,omitempty"`
	AllowedClientsFile   string        `json:"allowedClientsFile,omitempty"`
	MainCfFile           string        `json:"mainCfFile,omitempty"`
	Port                 string        `json:"port,omitempty"`
	AdminUsername        string        `json:"adminUsername,omitempty"`
	AdminPasswordHash    string        `json:"adminPasswordHash,omitempty"`
	MailLogFile          string        `json:"mailLogFile,omitempty"`
}

// System represents a client system allowed to relay mail.
type System struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Type     string `json:"type"`     // "internal" oder "external"
	Category string `json:"category"` // "printer","server","scanner","network","other"
}

// AppData is the persistent JSON data store.
type AppData struct {
	Systems        []System   `json:"systems"`
	BaseMynetworks string     `json:"baseMynetworks"`
	AllManagedIPs  []string   `json:"allManagedIps"`
	Config         *AppConfig `json:"config,omitempty"`
}

var (
	appData      AppData
	appMu        sync.Mutex // schützt appData und alle Postfix-Schreiboperationen
	dataFilePath string
)

func loadData() error {
	b, err := os.ReadFile(dataFilePath)
	if os.IsNotExist(err) {
		appData = AppData{}
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &appData); err != nil {
		return err
	}
	// Konfiguration aus data.json auf globale Variablen anwenden (überschreibt config.go-Defaults)
	if c := appData.Config; c != nil {
		relayServersInternal = c.RelayServersInternal
		relayServersExternal = c.RelayServersExternal
		if c.AllowedClientsFile != "" {
			allowedClientsFile = c.AllowedClientsFile
		}
		if c.MainCfFile != "" {
			mainCfFile = c.MainCfFile
		}
		if c.Port != "" {
			listenAddr = ":" + c.Port
		}
		if c.AdminUsername != "" {
			adminUsername = c.AdminUsername
		}
		if c.MailLogFile != "" {
			mailLogFile = c.MailLogFile
		}
	}
	return nil
}

// validateConfig prüft, ob alle Pflichtfelder in data.json gesetzt sind.
func validateConfig() error {
	if len(relayServersInternal) == 0 {
		return fmt.Errorf("config.relayServersInternal fehlt in data.json")
	}
	if len(relayServersExternal) == 0 {
		return fmt.Errorf("config.relayServersExternal fehlt in data.json")
	}
	if adminUsername == "" {
		return fmt.Errorf("config.adminUsername fehlt in data.json")
	}
	if appData.Config == nil || appData.Config.AdminPasswordHash == "" {
		return fmt.Errorf("config.adminPasswordHash fehlt in data.json")
	}
	return nil
}

// saveData speichert appData als JSON. Muss mit appMu gehalten aufgerufen werden.
func saveData() error {
	b, err := json.MarshalIndent(appData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFilePath, b, 0644)
}

// copyRelayServers gibt thread-sichere Kopien der Relay-Server-Slices zurück.
func copyRelayServers() (internal, external []RelayServer) {
	appMu.Lock()
	defer appMu.Unlock()
	internal = append([]RelayServer{}, relayServersInternal...)
	external = append([]RelayServer{}, relayServersExternal...)
	return
}

// addManagedIP fügt ip zu AllManagedIPs hinzu, sofern noch nicht vorhanden.
// Muss mit appMu gehalten aufgerufen werden.
func addManagedIP(ip string) {
	for _, existing := range appData.AllManagedIPs {
		if existing == ip {
			return
		}
	}
	appData.AllManagedIPs = append(appData.AllManagedIPs, ip)
}
