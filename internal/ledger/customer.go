package ledger

import (
	"localledger/internal/models"
)

// GetAllCustomers returns all active customers
func (l *Ledger) GetAllCustomers() ([]models.Customer, error) {
	rows, err := l.db.Query(`
		SELECT id, name, orgnr, address, created_at
		FROM customers
		ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []models.Customer
	for rows.Next() {
		var c models.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Orgnr, &c.Address, &c.CreatedAt); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, nil
}

// AnonymizeCustomer performs GDPR anonymization by wiping PII from the customer table
func (l *Ledger) AnonymizeCustomer(customerID int64) error {
	// WORM/BFL kräver att kopplingen från gamla fakturor finns kvar, men enligt GDPR tas datan bort.
	// Detta gör att vi "pseudonymiserar" namnet så fakturorna refererar till anonymiserad data, 
	// men eftersom den historiska PDF:en är frusen så påverkas inte originalverifikationen i arkivet.
	
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE customers 
		SET name = '[ANONYMISERAD]', orgnr = '', address = '' 
		WHERE id = ?`, customerID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE invoices 
		SET customer_name = '[ANONYMISERAD]', customer_orgnr = '', customer_address = '' 
		WHERE customer_id = ? AND status = 'utkast'`, customerID)
	if err != nil {
		return err
	}

	return tx.Commit()
}
