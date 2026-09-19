//go:build !linux

package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestOFDLockingUnavailable verifies that where OFD locks do not exist —
// every platform but Linux — the switch reports itself unavailable and changes
// nothing, both before and after a database file has been locked, and that
// OFDLockingEnabled never reports the mode as on.
func TestOFDLockingUnavailable(t *testing.T) {
	if OFDLockingEnabled() {
		t.Fatal("OFDLockingEnabled() = true on a platform without OFD locks")
	}
	for _, on := range []bool{true, false} {
		prev, err := OFDLocking(on)
		if prev != false || err != ErrOFDLockingUnavailable {
			t.Fatalf("OFDLocking(%v) = (%v, %v), want (false, ErrOFDLockingUnavailable)", on, prev, err)
		}
	}
	if OFDLockingEnabled() {
		t.Fatal("OFDLockingEnabled() = true after OFDLocking")
	}

	// Taking a database file lock does not change that.
	dbPath := filepath.Join(t.TempDir(), "ofd.db")
	db, err := sql.Open(DriverName, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE t(x)"); err != nil {
		t.Fatal(err)
	}
	if _, err := OFDLocking(true); err != ErrOFDLockingUnavailable {
		t.Fatalf("OFDLocking(true) after the first lock: err = %v, want ErrOFDLockingUnavailable", err)
	}
	if OFDLockingEnabled() {
		t.Fatal("OFDLockingEnabled() = true after the first lock")
	}
}
