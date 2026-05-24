package api

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecentWorkspace representerar en senast använd arbetsyta.
type RecentWorkspace struct {
	Path        string    `json:"path"`         // Kan sparas som relativ i filen, men returneras alltid som absolut till API:et
	CompanyName string    `json:"company_name"`
	LastUsed    time.Time `json:"last_used"`
	Orphaned    bool      `json:"orphaned"`     // Sätts dynamiskt vid laddning
}

// GlobalConfig representerar hela konfigurationsfilens struktur.
type GlobalConfig struct {
	RecentWorkspaces []RecentWorkspace `json:"recent_workspaces"`
}

func getExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exePath)
}

func makePathRelativeIfPossible(targetPath string) string {
	exeDir := getExeDir()
	if exeDir == "" {
		return targetPath
	}
	
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return targetPath
	}
	absExeDir, err := filepath.Abs(exeDir)
	if err != nil {
		return targetPath
	}
	
	// Om det ligger på en helt annan volym (t.ex. C: vs D:), spara absolut
	if filepath.VolumeName(absTarget) != filepath.VolumeName(absExeDir) {
		return absTarget
	}
	
	rel, err := filepath.Rel(absExeDir, absTarget)
	if err != nil {
		return absTarget
	}
	
	return rel
}

func makePathAbsolute(savedPath string) string {
	if filepath.IsAbs(savedPath) {
		return savedPath
	}
	exeDir := getExeDir()
	if exeDir == "" {
		return savedPath
	}
	return filepath.Join(exeDir, savedPath)
}

func getConfigPath() (string, bool) {
	exeDir := getExeDir()
	if exeDir != "" {
		path := filepath.Join(exeDir, "localledger_config.json")
		// Kontrollera om vi kan skriva till exe-katalogen (portabelt läge)
		testFile := filepath.Join(exeDir, ".config_write_test")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err == nil {
			os.Remove(testFile)
			return path, true
		}
	}
	
	// Om ej skrivbar exe-katalog, fall tillbaka till LOCALAPPDATA
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, ".localledger")
	}
	dir := filepath.Join(localAppData, "LocalLedger")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json"), false
}

// LoadGlobalConfig läser och parsar den globala konfigurationsfilen.
func LoadGlobalConfig() (GlobalConfig, string, bool) {
	configPath, isPortable := getConfigPath()
	var config GlobalConfig
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, configPath, isPortable
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Varning: Kunde inte parsa global config: %v", err)
		return config, configPath, isPortable
	}
	
	// Konvertera sparade relativa sökvägar till absoluta och validera Orphaned-status
	for i := range config.RecentWorkspaces {
		absPath := makePathAbsolute(config.RecentWorkspaces[i].Path)
		config.RecentWorkspaces[i].Path = absPath
		
		dbPath := filepath.Join(absPath, "ledger.db")
		_, errStat := os.Stat(dbPath)
		config.RecentWorkspaces[i].Orphaned = os.IsNotExist(errStat)
	}
	
	return config, configPath, isPortable
}

// SaveGlobalConfig lägger till eller uppdaterar en arbetsyta i konfigurationen.
func SaveGlobalConfig(workspacePath string, companyName string) error {
	if workspacePath == "" {
		return nil
	}
	
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		absWorkspace = workspacePath
	}
	
	config, configPath, _ := LoadGlobalConfig()
	
	foundIndex := -1
	for i, w := range config.RecentWorkspaces {
		if strings.EqualFold(w.Path, absWorkspace) {
			foundIndex = i
			break
		}
	}
	
	now := time.Now()
	newRecent := RecentWorkspace{
		Path:        absWorkspace,
		CompanyName: companyName,
		LastUsed:    now,
	}
	
	if foundIndex != -1 {
		if companyName == "" {
			newRecent.CompanyName = config.RecentWorkspaces[foundIndex].CompanyName
		}
		config.RecentWorkspaces = append(config.RecentWorkspaces[:foundIndex], config.RecentWorkspaces[foundIndex+1:]...)
	}
	
	config.RecentWorkspaces = append([]RecentWorkspace{newRecent}, config.RecentWorkspaces...)
	
	// Skapa sparbar kopia med relativa sökvägar där möjligt
	savedConfig := GlobalConfig{
		RecentWorkspaces: make([]RecentWorkspace, len(config.RecentWorkspaces)),
	}
	for i, w := range config.RecentWorkspaces {
		savedConfig.RecentWorkspaces[i] = RecentWorkspace{
			Path:        makePathRelativeIfPossible(w.Path),
			CompanyName: w.CompanyName,
			LastUsed:    w.LastUsed,
		}
	}
	
	data, err := json.MarshalIndent(savedConfig, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}

// RemoveGlobalConfig tar bort en arbetsyta från listan.
func RemoveGlobalConfig(workspacePath string) error {
	if workspacePath == "" {
		return nil
	}
	
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		absWorkspace = workspacePath
	}
	
	config, configPath, _ := LoadGlobalConfig()
	
	newRecent := make([]RecentWorkspace, 0, len(config.RecentWorkspaces))
	for _, w := range config.RecentWorkspaces {
		if !strings.EqualFold(w.Path, absWorkspace) {
			newRecent = append(newRecent, w)
		}
	}
	config.RecentWorkspaces = newRecent
	
	savedConfig := GlobalConfig{
		RecentWorkspaces: make([]RecentWorkspace, len(config.RecentWorkspaces)),
	}
	for i, w := range config.RecentWorkspaces {
		savedConfig.RecentWorkspaces[i] = RecentWorkspace{
			Path:        makePathRelativeIfPossible(w.Path),
			CompanyName: w.CompanyName,
			LastUsed:    w.LastUsed,
		}
	}
	
	data, err := json.MarshalIndent(savedConfig, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}
