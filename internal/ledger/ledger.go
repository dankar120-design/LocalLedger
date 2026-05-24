package ledger

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	_ "modernc.org/sqlite"
	
	"localledger/internal/models"
)

var (
	ErrDowngradeAttempt = errors.New("cannot open database: app version is older than required minimum version")
	ErrInvalidWorkspace = errors.New("invalid company workspace: ledger.db not found")
)

// Ledger representerar hjärnan i systemet och äger databasanslutningen.
type Ledger struct {
	db            *sql.DB
	isSandbox     bool
	appVersion    string
	workspacePath string
}

// OpenLedger öppnar ett Company Workspace och verifierar dess status och version.
func OpenLedger(workspacePath string, currentAppVersion string) (l *Ledger, err error) {
	// 1. Normalisera sökväg
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	dbPath := filepath.Join(absPath, "ledger.db")

	// 2. Kontrollera att filen finns
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, ErrInvalidWorkspace
	}

	// 3. Öppna anslutning med WAL, Foreign Keys, Busy Timeout och Immediate TxLock
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Säkrad resurshantering (Stänger anslutningen vid fel)
	success := false
	defer func() {
		if !success {
			db.Close()
		}
	}()

	// 3.5. Kör migreringar med Pre-Migration Backup
	backupPath := dbPath + ".tmp_backup"
	
	// Rensa eventuell gammal kvarlämnad backup
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to remove stale backup (file locked?): %w", err)
	}

	// Skapa atomär backup via VACUUM INTO
	safeBackupPath := strings.ReplaceAll(backupPath, "'", "''")
	backupQuery := fmt.Sprintf("VACUUM INTO '%s'", safeBackupPath)
	if _, err := db.Exec(backupQuery); err != nil {
		return nil, fmt.Errorf("failed to create pre-migration backup: %w", err)
	}

	// Utför migreringar
	if err := runMigrations(db); err != nil {
		// Stäng anslutningen för att släppa fil-låsningar (särskilt viktigt på Windows!)
		db.Close()
		
		// Ta bort halvmigrerade filer
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
		
		// Återställ från backup
		if renameErr := os.Rename(backupPath, dbPath); renameErr != nil {
			return nil, fmt.Errorf("migration failed (%v) AND database recovery failed: %v", err, renameErr)
		}
		
		return nil, fmt.Errorf("database migration failed; database was safely recovered to previous state: %w", err)
	}

	// Ta bort temporär backup vid lyckad migrering
	os.Remove(backupPath)

	// 4. Läs in schema_version
	var dbVersion, minAppVersion string
	var isSandbox bool
	err = db.QueryRow("SELECT version, app_min_version, is_sandbox FROM schema_version LIMIT 1").Scan(&dbVersion, &minAppVersion, &isSandbox)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema version: %w", err)
	}

	// 5. Strikt Semver-validering och Downgrade Protection
	if !semver.IsValid(currentAppVersion) || !semver.IsValid(minAppVersion) {
		return nil, fmt.Errorf("invalid semver format: app=%q, db_min=%q", currentAppVersion, minAppVersion)
	}

	if semver.Compare(currentAppVersion, minAppVersion) == -1 {
		return nil, ErrDowngradeAttempt
	}

	success = true
	return &Ledger{
		db:            db,
		isSandbox:     isSandbox,
		appVersion:    currentAppVersion,
		workspacePath: absPath,
	}, nil
}

// Close stänger databasanslutningen.
func (l *Ledger) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// DB returnerar den underliggande databasanslutningen.
func (l *Ledger) DB() *sql.DB {
	return l.db
}

// IsSandbox returnerar true om detta workspace är i sandlådeläge.
func (l *Ledger) IsSandbox() bool {
	return l.isSandbox
}

// WorkspacePath returnerar sökvägen till den aktiva arbetsytan.
func (l *Ledger) WorkspacePath() string {
	return l.workspacePath
}

// ExportSnapshot tar en 100% säker ögonblicksbild av databasen till den givna sökvägen.
func (l *Ledger) ExportSnapshot(targetPath string) error {
	// I SQLite (och modernc/sqlite) kopierar VACUUM INTO hela databasen 
	// (inklusive pågående data i WAL-filen) till en ny, konsoliderad fil.
	// Byt ut eventuella enkelfnuttar för att förhindra SQL-injektion.
	safePath := strings.ReplaceAll(targetPath, "'", "''")
	query := fmt.Sprintf("VACUUM INTO '%s'", safePath)
	_, err := l.db.Exec(query)
	return err
}

// GetActiveFiscalYear returnerar det senaste öppna räkenskapsåret.
func (l *Ledger) GetActiveFiscalYear() (*models.FiscalYear, error) {
	var fy models.FiscalYear
	// Vi antar att det högsta ID:t som inte är låst är det aktuella
	err := l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE locked_at IS NULL ORDER BY id DESC LIMIT 1").Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoFiscalYear
		}
		return nil, err
	}
	return &fy, nil
}

// GetAccounts returnerar alla konton i kontoplanen.
func (l *Ledger) GetAccounts() ([]models.Account, error) {
	rows, err := l.db.Query("SELECT code, name, type, COALESCE(sru_code, '') FROM accounts ORDER BY code ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		if err := rows.Scan(&a.Code, &a.Name, &a.Type, &a.SRUCode); err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	
	return accounts, nil
}

// GetDistinctVendors hämtar alla unika beskrivningar (vendors) från tidigare verifikationer,
// filtrerar bort tomma strängar och korta strängar (< 4 tecken).
func (l *Ledger) GetDistinctVendors() ([]string, error) {
	query := `
		SELECT DISTINCT text 
		FROM verifications 
		WHERE text != '' AND length(text) >= 4
	`
	rows, err := l.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct vendors: %w", err)
	}
	defer rows.Close()

	var vendors []string
	for rows.Next() {
		var vendor string
		if err := rows.Scan(&vendor); err != nil {
			return nil, fmt.Errorf("failed to scan vendor: %w", err)
		}
		vendors = append(vendors, vendor)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in GetDistinctVendors: %w", err)
	}

	return vendors, nil
}

// GetSuggestedAccountForVendor hämtar det senast använda kostnads- eller intäktskontot för en given leverantör/text.
func (l *Ledger) GetSuggestedAccountForVendor(vendor string) (string, error) {
	if vendor == "" {
		return "", nil
	}
	query := `
		SELECT r.account 
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		WHERE v.text LIKE ?
		  AND r.account NOT LIKE '19%' 
		  AND r.account NOT LIKE '24%' 
		  AND r.account NOT LIKE '26%'
		  AND r.account NOT LIKE '15%'
		ORDER BY v.date DESC, v.id DESC
		LIMIT 1
	`
	var account string
	err := l.db.QueryRow(query, "%"+vendor+"%").Scan(&account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get suggested account for vendor: %w", err)
	}
	return account, nil
}

