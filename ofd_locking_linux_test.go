//go:build linux

package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// fOfdSetlk is F_OFD_SETLK, the Open File Description variant of F_SETLK. The
// syscall package does not export it on every architecture.
const fOfdSetlk = 37

// ofdChildEnvVar marks the child process a test re-executed so that it starts
// with a process that has not locked any database file yet and can therefore
// still switch the locking mode; see reexecOFDChild.
const ofdChildEnvVar = "QIST_SQLITE_TEST_OFD_CHILD"

// reexecOFDChild re-runs the calling test alone in a child process. The
// locking mode is process-wide and frozen at the process's first database file
// lock, so a test cannot enable it inside the test binary the rest of the
// suite runs in. A skipped child skips the caller; a failed one fails it.
func reexecOFDChild(t *testing.T) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// Drop MODERNC_SQLITE_OFD_LOCK from the inherited environment so the child
	// starts with OFD locking off and the Go switch is what turns it on.
	env := []string{ofdChildEnvVar + "=1"}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "MODERNC_SQLITE_OFD_LOCK=") {
			continue
		}
		env = append(env, kv)
	}

	cmd := exec.Command(exe, "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child process: %v\n%s", err, out)
	}
	switch {
	case bytes.Contains(out, []byte("--- SKIP")):
		t.Skipf("child process skipped:\n%s", out)
	case !bytes.Contains(out, []byte("--- PASS")):
		t.Fatalf("child process did not run the test:\n%s", out)
	}
}

// TestOFDLockingEnablesOFDLocks verifies the Go switch really turns Linux OFD
// locks on. It runs in a child process, because the mode must be set before
// the first database file lock of the process, which the rest of the suite has
// long since taken. In the child it verifies that OFDLocking(true) reports the
// previous mode and flips OFDLockingEnabled, and that the lock SQLite's write
// transaction holds is an OFD lock: it must survive closing an unrelated
// descriptor of the same database file, which strips POSIX locks.
func TestOFDLockingEnablesOFDLocks(t *testing.T) {
	if os.Getenv(ofdChildEnvVar) == "" {
		reexecOFDChild(t)
		return
	}

	// Fresh process: nothing has locked a database file here yet.
	if OFDLockingEnabled() {
		t.Fatal("OFD locking is on before OFDLocking(true)")
	}

	prev, err := OFDLocking(true)
	if err == ErrOFDLockingUnavailable {
		t.Skip("OFD locks unavailable on this kernel/filesystem")
	}
	if err != nil {
		t.Fatalf("OFDLocking(true): %v", err)
	}
	if prev {
		t.Fatalf("OFDLocking(true) returned prev = true, want false")
	}
	if !OFDLockingEnabled() {
		t.Fatal("OFDLockingEnabled() = false right after OFDLocking(true)")
	}

	dbPath := filepath.Join(t.TempDir(), "ofd.db")
	db, err := sql.Open(DriverName, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// The first lock attempt on the database file: on a kernel or filesystem
	// that rejects OFD locks the library falls back to POSIX locks here.
	if _, err := conn.ExecContext(ctx, "CREATE TABLE t(x); INSERT INTO t VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if !OFDLockingEnabled() {
		t.Skip("kernel or filesystem rejected OFD locks; POSIX fallback in effect")
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO t VALUES(2)"); err != nil {
		t.Fatal(err)
	}

	// Closing an unrelated descriptor of the same inode strips POSIX locks;
	// an OFD lock is attached to SQLite's own open file description and stays.
	fd, err := syscall.Open(dbPath, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Close(fd); err != nil {
		t.Fatal(err)
	}

	// A conflicting write lock from an independent descriptor must be refused
	// while SQLite holds the write transaction. Under POSIX locks this probe
	// would be granted, since POSIX locks are owned by the process itself and
	// never conflict with it.
	probe, err := syscall.Open(dbPath, syscall.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(probe)

	fl := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 0, Len: 0}
	switch err := syscall.FcntlFlock(uintptr(probe), fOfdSetlk, &fl); err {
	case syscall.EINVAL:
		t.Skip("F_OFD_SETLK unsupported on this kernel/filesystem")
	case syscall.EAGAIN, syscall.EACCES:
		// Expected: SQLite holds a conflicting OFD lock.
	case nil:
		t.Fatal("kernel granted a conflicting write lock; SQLite does not hold an OFD lock")
	default:
		t.Fatalf("lock probe: %v", err)
	}

	// With the mode in effect since the first lock, it can no longer be changed.
	if _, err := OFDLocking(false); err != ErrOFDLockingTooLate {
		t.Fatalf("OFDLocking(false) after the first lock: err = %v, want ErrOFDLockingTooLate", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// TestOFDLockingFrozenAfterFirstLock verifies the contract of an OFD locking
// call made once the process has already locked a database file: the mode is
// frozen, so changing it fails with ErrOFDLockingTooLate, while a call that
// asks for the mode already in effect and the query keep working. The mode is
// frozen by this test itself, so it does not depend on which other tests ran
// before it.
func TestOFDLockingFrozenAfterFirstLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ofd_frozen.db")
	db, err := sql.Open(DriverName, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE t(x); INSERT INTO t VALUES(1)"); err != nil {
		t.Fatal(err)
	}

	cur := OFDLockingEnabled()
	switch _, err := OFDLocking(!cur); err {
	case ErrOFDLockingTooLate:
		// Expected.
	case ErrOFDLockingUnavailable:
		// MODERNC_SQLITE_OFD_LOCK was requested in the environment and this
		// kernel or filesystem rejected OFD locks.
		t.Skip("OFD locks unavailable on this kernel/filesystem")
	default:
		t.Fatalf("OFDLocking(%v) after the first lock: err = %v, want ErrOFDLockingTooLate", !cur, err)
	}

	if prev, err := OFDLocking(cur); prev != cur || err != nil {
		t.Fatalf("no-change OFDLocking(%v): prev = %v, err = %v, want %v, nil", cur, prev, err, cur)
	}
	if got := OFDLockingEnabled(); got != cur {
		t.Fatalf("OFDLockingEnabled() = %v, want %v", got, cur)
	}
}
