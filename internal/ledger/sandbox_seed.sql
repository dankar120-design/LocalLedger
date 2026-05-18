-- Fyller huvudboken med testdata (accounts är redan populerade av InitWorkspace)

-- Skapa några test-verifikationer
INSERT INTO verifications (date, text, type) VALUES 
('2026-01-15', 'Aktiekapital', 'NORMAL'),
('2026-01-20', 'Lokalhyra Januari', 'NORMAL'),
('2026-02-05', 'Försäljning Webbutik', 'NORMAL'),
('2026-12-31', 'Bokslut 2026 (Årets resultat)', 'NORMAL'),
('2026-03-10', 'Vercel Hosting (EU-tjänst)', 'NORMAL'),
('2026-03-15', 'Representation (Lunchkund)', 'NORMAL');

INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES 
(1, '1930', 50000, 0),
(1, '2010', 0, 50000),
(2, '5010', 5000, 0),
(2, '1930', 0, 5000),
(3, '1930', 12500, 0),
(3, '2611', 0, 2500),
(3, '3001', 0, 10000),
(4, '8999', 5000, 0),
(4, '2099', 0, 5000),
(5, '1930', 0, 1000),
(5, '4531', 1000, 0),
(5, '2645', 250, 0),
(5, '2614', 0, 250),
(6, '1930', 0, 850),
(6, '6071', 680, 0),
(6, '2641', 170, 0);

-- Falsk faktura
INSERT INTO invoices (
    invoice_number, date, due_date, payment_terms_days, customer_name, customer_orgnr, 
    customer_address, total_amount, total_vat, status, fiscal_year_id
) VALUES (
    '10001', '2026-03-15', '2026-04-14', 30, 'DemoFöretaget AB', '556677-8899', 
    'Testgatan 1, 123 45 Teststad', 12500, 2500, 'sent', 1
);

INSERT INTO invoice_items (invoice_id, description, quantity, price_ex_vat, vat_rate) VALUES 
(1, 'Konsulttimmar Sandbox', 10, 1000, 25);
