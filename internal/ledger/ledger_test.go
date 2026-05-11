package ledger

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// setupTestWorkspace kopierar vår sandbox-databas till en temporär mapp för isolerade tester.
func setupTestWorkspace(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	
	baseSrcPath := filepath.Join("..", "..", "examples", "DemoForetaget_AB", "ledger.db")
	baseDstPath := filepath.Join(tempDir, "ledger.db")

	copyFile := func(src, dst string) {
		s, err := os.Open(src)
		if err != nil {
			if os.IsNotExist(err) {
				return // WAL/SHM might not exist, which is fine
			}
			t.Fatalf("failed to open source db file %s: %v", src, err)
		}
		defer s.Close()

		d, err := os.Create(dst)
		if err != nil {
			t.Fatalf("failed to create temp db file %s: %v", dst, err)
		}
		defer d.Close()

		if _, err = io.Copy(d, s); err != nil {
			t.Fatalf("failed to copy file %s to %s: %v", src, dst, err)
		}
	}

	copyFile(baseSrcPath, baseDstPath)
	copyFile(baseSrcPath+"-wal", baseDstPath+"-wal")
	copyFile(baseSrcPath+"-shm", baseDstPath+"-shm")

	return tempDir
}

func TestOpenLedger(t *testing.T) {
	t.Run("Success opening sandbox", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		
		l, err := OpenLedger(workspace, "v1.4.0")
		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
		defer l.Close()

		if !l.IsSandbox() {
			t.Error("Expected Ledger to be in sandbox mode")
		}
	})

	t.Run("Downgrade is blocked", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		
		_, err := OpenLedger(workspace, "v0.9.0")
		
		if !errors.Is(err, ErrDowngradeAttempt) {
			t.Errorf("Expected ErrDowngradeAttempt, got: %v", err)
		}
	})

	t.Run("Invalid path returns error", func(t *testing.T) {
		_, err := OpenLedger("./non-existent-folder", "v1.4.0")
		if !errors.Is(err, ErrInvalidWorkspace) {
			t.Errorf("Expected ErrInvalidWorkspace, got: %v", err)
		}
	})
}
