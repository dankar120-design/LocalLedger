package ledger

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// InitWorkspace skapar en ny, tom produktionsdatabas i den angivna mappen.
func InitWorkspace(workspacePath string) error {
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	attachmentsPath := filepath.Join(absPath, "attachments")
	if err := os.MkdirAll(attachmentsPath, 0755); err != nil {
		return fmt.Errorf("failed to create attachments directory: %w", err)
	}

	dbPath := filepath.Join(absPath, "ledger.db")

	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("database already exists at %s", dbPath)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping new database: %w", err)
	}

	// Låt OpenLedger hantera migreringar
	return nil
}

// runMigrations applicerar saknade SQL-schema uppdateringar sekventiellt och transaktionellt.
func runMigrations(db *sql.DB) error {
	// Baseline-detektion: Om accounts finns men INTE schema_migrations
	var accountsExists, migrationsExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='accounts')").Scan(&accountsExists)
	if err != nil {
		return fmt.Errorf("failed to check for baseline: %w", err)
	}
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='schema_migrations')").Scan(&migrationsExists)
	if err != nil {
		return fmt.Errorf("failed to check for schema_migrations: %w", err)
	}

	// Om vi har data men ingen migrations-tabell, måste vi fejka Version 1
	if accountsExists && !migrationsExists {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)")
		if err != nil {
			return fmt.Errorf("failed to create baseline schema_migrations: %w", err)
		}
		_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (1)")
		if err != nil {
			return fmt.Errorf("failed to insert baseline version: %w", err)
		}
	} else if !migrationsExists {
		// Helt ny databas
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)")
		if err != nil {
			return fmt.Errorf("failed to create schema_migrations: %w", err)
		}
	}

	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to read current schema version: %w", err)
	}

	migrations := []string{
		// Version 1: Baseline
		`
		CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			version TEXT NOT NULL,
			app_min_version TEXT NOT NULL,
			is_sandbox BOOLEAN NOT NULL
		);
		CREATE TABLE IF NOT EXISTS accounts (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS fiscal_years (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			locked_at TEXT
		);
		CREATE TABLE IF NOT EXISTS period_locks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			year_month TEXT NOT NULL UNIQUE,
			locked_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL DEFAULT (datetime('now', 'localtime')),
			user TEXT NOT NULL,
			action TEXT NOT NULL,
			details TEXT
		);
		CREATE TABLE IF NOT EXISTS verifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime')),
			date TEXT NOT NULL,
			text TEXT NOT NULL,
			hash TEXT CHECK(hash IS NULL OR length(hash) = 64)
		);
		CREATE TABLE IF NOT EXISTS verification_rows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			verification_id INTEGER NOT NULL,
			account TEXT NOT NULL,
			debet INTEGER NOT NULL DEFAULT 0,
			kredit INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(verification_id) REFERENCES verifications(id),
			FOREIGN KEY(account) REFERENCES accounts(code),
			CHECK (debet >= 0 AND kredit >= 0 AND NOT (debet > 0 AND kredit > 0))
		);
		CREATE TRIGGER IF NOT EXISTS protect_sealed_verifications BEFORE UPDATE ON verifications FOR EACH ROW WHEN OLD.hash IS NOT NULL BEGIN SELECT RAISE(FAIL, 'WORM Violation: Cannot update a sealed verification.'); END;
		CREATE TRIGGER IF NOT EXISTS protect_sealed_verifications_delete BEFORE DELETE ON verifications FOR EACH ROW WHEN OLD.hash IS NOT NULL BEGIN SELECT RAISE(FAIL, 'WORM Violation: Cannot delete a sealed verification.'); END;
		CREATE TRIGGER IF NOT EXISTS protect_sealed_verification_rows_update BEFORE UPDATE ON verification_rows FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT hash FROM verifications WHERE id = OLD.verification_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot update rows of a sealed verification.') END; END;
		CREATE TRIGGER IF NOT EXISTS protect_sealed_verification_rows_insert BEFORE INSERT ON verification_rows FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT hash FROM verifications WHERE id = NEW.verification_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot insert rows into a sealed verification.') END; END;
		CREATE TRIGGER IF NOT EXISTS protect_sealed_verification_rows_delete BEFORE DELETE ON verification_rows FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT hash FROM verifications WHERE id = OLD.verification_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot delete rows from a sealed verification.') END; END;
		
		INSERT INTO schema_version (id, version, app_min_version, is_sandbox) VALUES (1, 'v1.0.0', 'v1.0.0', false) ON CONFLICT(id) DO NOTHING;
		INSERT INTO accounts (code, name, type) VALUES 
			('1510', 'Kundfordringar', 'Tillgång'),
			('1630', 'Avräkning för skatter och avgifter (skattekonto)', 'Tillgång'),
			('1910', 'Kassa', 'Tillgång'),
			('1930', 'Företagskonto/checkkonto', 'Tillgång'),
			('2010', 'Eget kapital', 'Skuld'),
			('2013', 'Övriga egna uttag', 'Skuld'),
			('2018', 'Övriga egna insättningar', 'Skuld'),
			('2440', 'Leverantörsskulder', 'Skuld'),
			('2610', 'Utgående moms, oreducerad (25%)', 'Skuld'),
			('2611', 'Utgående moms på försäljning inom Sverige, 25 %', 'Skuld'),
			('2620', 'Utgående moms, reducerad (12%)', 'Skuld'),
			('2630', 'Utgående moms, reducerad (6%)', 'Skuld'),
			('2640', 'Ingående moms', 'Skuld'),
			('2641', 'Debiterad ingående moms', 'Skuld'),
			('2650', 'Redovisningskonto för moms', 'Skuld'),
			('3000', 'Försäljning inom Sverige', 'Intäkt'),
			('3001', 'Försäljning inom Sverige, 25 %', 'Intäkt'),
			('3040', 'Försäljning tjänster (Sverige)', 'Intäkt'),
			('4000', 'Inköp av varor från Sverige', 'Kostnad'),
			('4010', 'Inköp varor', 'Kostnad'),
			('5010', 'Lokalhyra', 'Kostnad'),
			('5410', 'Förbrukningsinventarier', 'Kostnad'),
			('5420', 'Programvaror', 'Kostnad'),
			('5800', 'Resekostnader', 'Kostnad'),
			('6071', 'Representation, avdragsgill', 'Kostnad'),
			('6110', 'Kontorsmateriel', 'Kostnad'),
			('6310', 'Företagsförsäkringar', 'Kostnad'),
			('6570', 'Bankkostnader', 'Kostnad'),
			('6990', 'Övriga externa kostnader', 'Kostnad') ON CONFLICT(code) DO NOTHING;
		INSERT INTO fiscal_years (start_date, end_date) SELECT strftime('%Y-01-01', 'now', 'localtime'), strftime('%Y-12-31', 'now', 'localtime') WHERE NOT EXISTS(SELECT 1 FROM fiscal_years);
		`,
		// Version 2: Storno Reference ID
		`
		ALTER TABLE verifications ADD COLUMN storno_ref_id INTEGER REFERENCES verifications(id);
		UPDATE schema_version SET version = 'v1.1.0';
		`,
		// Version 3: Storno Reference Index & App Min Version
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_storno_ref_id ON verifications(storno_ref_id) WHERE storno_ref_id IS NOT NULL;
		UPDATE schema_version SET app_min_version = 'v1.1.0';
		`,
		// Version 4: WORM Attachments (Content-Addressable Storage)
		`
		ALTER TABLE verifications ADD COLUMN attachment_hash TEXT;
		ALTER TABLE verifications ADD COLUMN attachment_mime TEXT;
		UPDATE schema_version SET version = 'v1.2.0', app_min_version = 'v1.2.0';
		`,
		// Version 5: Utökad BAS K1-kontoplan
		`
		INSERT INTO accounts (code, name, type) VALUES 
			('1510', 'Kundfordringar', 'Tillgång'),
			('1630', 'Avräkning för skatter och avgifter (skattekonto)', 'Tillgång'),
			('1910', 'Kassa', 'Tillgång'),
			('1930', 'Företagskonto/checkkonto', 'Tillgång'),
			('2010', 'Eget kapital', 'Skuld'),
			('2013', 'Övriga egna uttag', 'Skuld'),
			('2018', 'Övriga egna insättningar', 'Skuld'),
			('2440', 'Leverantörsskulder', 'Skuld'),
			('2610', 'Utgående moms, oreducerad (25%)', 'Skuld'),
			('2611', 'Utgående moms på försäljning inom Sverige, 25 %', 'Skuld'),
			('2620', 'Utgående moms, reducerad (12%)', 'Skuld'),
			('2630', 'Utgående moms, reducerad (6%)', 'Skuld'),
			('2640', 'Ingående moms', 'Skuld'),
			('2641', 'Debiterad ingående moms', 'Skuld'),
			('2650', 'Redovisningskonto för moms', 'Skuld'),
			('3000', 'Försäljning inom Sverige', 'Intäkt'),
			('3001', 'Försäljning inom Sverige, 25 %', 'Intäkt'),
			('3040', 'Försäljning tjänster (Sverige)', 'Intäkt'),
			('4000', 'Inköp av varor från Sverige', 'Kostnad'),
			('4010', 'Inköp varor', 'Kostnad'),
			('5010', 'Lokalhyra', 'Kostnad'),
			('5410', 'Förbrukningsinventarier', 'Kostnad'),
			('5420', 'Programvaror', 'Kostnad'),
			('5800', 'Resekostnader', 'Kostnad'),
			('6071', 'Representation, avdragsgill', 'Kostnad'),
			('6110', 'Kontorsmateriel', 'Kostnad'),
			('6310', 'Företagsförsäkringar', 'Kostnad'),
			('6570', 'Bankkostnader', 'Kostnad'),
			('6990', 'Övriga externa kostnader', 'Kostnad') ON CONFLICT(code) DO NOTHING;
		UPDATE schema_version SET version = 'v1.2.1', app_min_version = 'v1.2.0';
		`,
		// Version 6: Företagsinställningar
		`
		CREATE TABLE IF NOT EXISTS company_settings (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			name TEXT NOT NULL DEFAULT '',
			org_number TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO company_settings (id, name, org_number) VALUES (1, '', '') ON CONFLICT(id) DO NOTHING;
		UPDATE schema_version SET version = 'v1.3.0', app_min_version = 'v1.3.0';
		`,
		// Version 7: Audit Compliance (locked_by)
		`
		ALTER TABLE period_locks ADD COLUMN locked_by TEXT NOT NULL DEFAULT 'System';
		UPDATE schema_version SET version = 'v1.4.0', app_min_version = 'v1.3.0';
		`,
		// Version 8: BFL Compliance (IB)
		`
		ALTER TABLE verifications ADD COLUMN type TEXT NOT NULL DEFAULT 'NORMAL';
		UPDATE schema_version SET version = 'v1.5.0', app_min_version = 'v1.4.0';
		`,
		// Version 9: Phase 2 - Inbox & Cloud Fetch
		`
		CREATE TABLE IF NOT EXISTS inbox_items (
			id TEXT PRIMARY KEY,
			original_filename TEXT NOT NULL,
			stored_filename TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			source TEXT DEFAULT 'local'
		);
		ALTER TABLE company_settings ADD COLUMN cloud_inbox_path TEXT NOT NULL DEFAULT '';
		UPDATE schema_version SET version = 'v2.0.0', app_min_version = 'v1.5.0';
		`,
		// Version 10: Phase 6 - Inbyggd Fakturering (Invoices)
		`
		ALTER TABLE company_settings ADD COLUMN address TEXT NOT NULL DEFAULT '';
		ALTER TABLE company_settings ADD COLUMN bankgiro TEXT NOT NULL DEFAULT '';
		ALTER TABLE company_settings ADD COLUMN swish_number TEXT NOT NULL DEFAULT '';
		ALTER TABLE company_settings ADD COLUMN invoice_start_number INTEGER NOT NULL DEFAULT 1000;
		ALTER TABLE company_settings ADD COLUMN payment_terms_days INTEGER NOT NULL DEFAULT 30;
		ALTER TABLE company_settings ADD COLUMN logo_path TEXT NOT NULL DEFAULT '';
		
		CREATE TABLE IF NOT EXISTS invoices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invoice_number TEXT UNIQUE,
			date TEXT NOT NULL,
			due_date TEXT NOT NULL,
			payment_terms_days INTEGER DEFAULT 30,
			customer_name TEXT NOT NULL,
			customer_orgnr TEXT,
			customer_address TEXT,
			total_amount INTEGER NOT NULL,
			total_vat INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'utkast',
			verification_id INTEGER REFERENCES verifications(id),
			credit_of INTEGER REFERENCES invoices(id),
			fiscal_year_id INTEGER NOT NULL REFERENCES fiscal_years(id),
			created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime'))
		);

		CREATE TABLE IF NOT EXISTS invoice_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invoice_id INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
			description TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			price_ex_vat INTEGER NOT NULL,
			vat_rate INTEGER NOT NULL
		);

		UPDATE schema_version SET version = 'v3.0.0', app_min_version = 'v2.0.0';
		`,
		// Version 11: Phase 7 - Bokslut & IB (Saknade BAS-konton)
		`
		INSERT INTO accounts (code, name, type) VALUES 
			('2019', 'Årets resultat, enskild näringsidkare', 'Skuld'),
			('2091', 'Balanserad vinst eller förlust', 'Skuld'),
			('2098', 'Vinst eller förlust från föregående år', 'Skuld'),
			('2099', 'Årets resultat (EK)', 'Skuld'),
			('8999', 'Årets resultat', 'Kostnad') ON CONFLICT(code) DO NOTHING;
		UPDATE schema_version SET version = 'v3.1.0', app_min_version = 'v2.0.0';
		`,
		// Version 12: Nya BAS-konton för strikt momsmappning (Ruta 06, 07, 20, 21, 30, 48)
		`
		INSERT INTO accounts (code, name, type) VALUES 
			('2614', 'Utgående moms omvänd skattskyldighet, 25 %', 'Skuld'),
			('2645', 'Ingående moms på förvärv från utlandet', 'Skuld'),
			('3002', 'Försäljning inom Sverige, 12 %', 'Intäkt'),
			('3003', 'Försäljning inom Sverige, 6 %', 'Intäkt'),
			('3042', 'Försäljning tjänster (Sverige), 12 %', 'Intäkt'),
			('3043', 'Försäljning tjänster (Sverige), 6 %', 'Intäkt'),
			('4515', 'Inköp av varor från annat EU-land, 25 %', 'Kostnad'),
			('4531', 'Inköp av tjänster från annat EU-land, 25 %', 'Kostnad') ON CONFLICT(code) DO NOTHING;
		UPDATE schema_version SET version = 'v3.2.0', app_min_version = 'v2.0.0';
		`,
		// Version 13: Standardkonton för varuförsäljning inrikes 25% (Ruta 05)
		`
		INSERT INTO accounts (code, name, type) VALUES 
			('3010', 'Försäljning varor', 'Intäkt'),
			('3011', 'Försäljning varor inom Sverige, 25 %', 'Intäkt'),
			('3020', 'Försäljning tjänster', 'Intäkt') ON CONFLICT(code) DO NOTHING;
		UPDATE schema_version SET version = 'v3.3.0', app_min_version = 'v2.0.0';
		`,
		// Version 14: Phase 8 - Kundregister & GDPR
		`
		CREATE TABLE IF NOT EXISTS customers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			orgnr TEXT,
			address TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime'))
		);

		INSERT INTO customers (name, orgnr, address)
		SELECT DISTINCT customer_name, customer_orgnr, customer_address
		FROM invoices
		WHERE customer_name != '' AND customer_name IS NOT NULL
		GROUP BY customer_name;

		ALTER TABLE invoices ADD COLUMN customer_id INTEGER REFERENCES customers(id);

		UPDATE invoices SET customer_id = (SELECT id FROM customers WHERE customers.name = invoices.customer_name);
		
		UPDATE schema_version SET version = 'v3.4.0', app_min_version = 'v2.0.0';
		`,
		// Version 15: Phase 9 - E2E Audit & Hardening (Invoices WORM Triggers)
		`
		CREATE TRIGGER IF NOT EXISTS protect_posted_invoices_delete BEFORE DELETE ON invoices FOR EACH ROW WHEN OLD.verification_id IS NOT NULL OR OLD.status != 'utkast' BEGIN SELECT RAISE(FAIL, 'WORM Violation: Cannot delete a posted invoice.'); END;
		
		CREATE TRIGGER IF NOT EXISTS protect_posted_invoices_update BEFORE UPDATE ON invoices FOR EACH ROW WHEN OLD.verification_id IS NOT NULL OR OLD.status != 'utkast' BEGIN SELECT CASE WHEN NEW.invoice_number IS NOT OLD.invoice_number OR NEW.date != OLD.date OR NEW.due_date != OLD.due_date OR NEW.payment_terms_days IS NOT OLD.payment_terms_days OR NEW.customer_id IS NOT OLD.customer_id OR NEW.customer_name != OLD.customer_name OR NEW.customer_orgnr IS NOT OLD.customer_orgnr OR NEW.customer_address IS NOT OLD.customer_address OR NEW.total_amount != OLD.total_amount OR NEW.total_vat != OLD.total_vat OR NEW.verification_id IS NOT OLD.verification_id OR NEW.credit_of IS NOT OLD.credit_of OR NEW.fiscal_year_id != OLD.fiscal_year_id THEN RAISE(FAIL, 'WORM Violation: Cannot modify accounting details of a posted invoice.') END; END;

		CREATE TRIGGER IF NOT EXISTS protect_posted_invoice_items_update BEFORE UPDATE ON invoice_items FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT verification_id FROM invoices WHERE id = OLD.invoice_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot update items of a posted invoice.') END; END;
		CREATE TRIGGER IF NOT EXISTS protect_posted_invoice_items_insert BEFORE INSERT ON invoice_items FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT verification_id FROM invoices WHERE id = NEW.invoice_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot insert items into a posted invoice.') END; END;
		CREATE TRIGGER IF NOT EXISTS protect_posted_invoice_items_delete BEFORE DELETE ON invoice_items FOR EACH ROW BEGIN SELECT CASE WHEN (SELECT verification_id FROM invoices WHERE id = OLD.invoice_id) IS NOT NULL THEN RAISE(FAIL, 'WORM Violation: Cannot delete items from a posted invoice.') END; END;

		UPDATE schema_version SET version = 'v3.5.0', app_min_version = 'v2.0.0';
		`,
		// Version 16: Lägg till saknade BAS-konton för 12%, 6% och 0% moms (2621, 2631, 3004)
		`
		INSERT INTO accounts (code, name, type) VALUES 
			('2621', 'Utgående moms på försäljning inom Sverige, 12 %', 'Skuld'),
			('2631', 'Utgående moms på försäljning inom Sverige, 6 %', 'Skuld'),
			('3004', 'Momsfri försäljning inom Sverige', 'Intäkt') ON CONFLICT(code) DO NOTHING;
		UPDATE schema_version SET version = 'v3.6.0', app_min_version = 'v2.0.0';
		`,
		// Version 17: BAS 2026 SRU Mappings Support
		`
		ALTER TABLE accounts ADD COLUMN sru_code TEXT;
		UPDATE schema_version SET version = 'v3.7.0', app_min_version = 'v2.0.0';
		`,
	}

	for i := currentVersion; i < len(migrations); i++ {
		versionToApply := i + 1
		
		// Kör inuti en transaktion
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin migration tx: %w", err)
		}

		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration to version %d failed: %w", versionToApply, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", versionToApply); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration version %d: %w", versionToApply, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", versionToApply, err)
		}
	}

	return nil
}
