package api

import (
	"bytes"
	"testing"
)

func TestBackupEncryptionDecryption(t *testing.T) {
	password := "StriktLegaltLösenord123"
	payload := []byte("Detta är ett superhemligt testmeddelande för LocalLedger WORM-backup.")

	// 1. Kryptera payload
	encrypted, err := encryptPayload(payload, password)
	if err != nil {
		t.Fatalf("failed to encrypt payload: %v", err)
	}

	// Kontrollera att signaturen finns
	if !bytes.HasPrefix(encrypted, []byte(cryptSignature)) {
		t.Errorf("encrypted payload missing signature prefix")
	}

	// 2. Dekryptera med rätt lösenord
	decrypted, err := decryptPayload(encrypted, password)
	if err != nil {
		t.Fatalf("failed to decrypt payload with correct password: %v", err)
	}

	if !bytes.Equal(decrypted, payload) {
		t.Errorf("decrypted payload mismatch; expected %q, got %q", payload, decrypted)
	}

	// 3. Försök dekryptera med fel lösenord
	_, err = decryptPayload(encrypted, "FelLösenord")
	if err == nil {
		t.Errorf("expected decryption to fail with incorrect password, but it succeeded")
	}

	// 4. Försök dekryptera skadad data
	corruptEncrypted := make([]byte, len(encrypted))
	copy(corruptEncrypted, encrypted)
	if len(corruptEncrypted) > 40 {
		corruptEncrypted[40] ^= 0xFF // Flippa en bit i ciphertexten
	}
	_, err = decryptPayload(corruptEncrypted, password)
	if err == nil {
		t.Errorf("expected decryption to fail with corrupted payload, but it succeeded")
	}
}
