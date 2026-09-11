package sqlite

import (
	msqlite "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Linux Open File Description (OFD) locks (modernc.org/sqlite v1.58.0+)
// ---------------------------------------------------------------------------

// ErrOFDLockingUnavailable is returned by OFDLocking where OFD locks are not
// available: on every platform but Linux, and on Linux after the kernel or
// filesystem has rejected them and the library has fallen back to POSIX locks
// for the remainder of the process.
var ErrOFDLockingUnavailable = msqlite.ErrOFDLockingUnavailable

// ErrOFDLockingTooLate is returned by OFDLocking when the locking mode can no
// longer be changed: a database file lock has already been attempted in this
// process, and from the first lock attempt on the mode is fixed. Enable OFD
// locking before the first connection is opened.
var ErrOFDLockingTooLate = msqlite.ErrOFDLockingTooLate

// OFDLocking switches SQLite between POSIX record locks (the default) and
// Linux Open File Description (OFD) locks for database files, process-wide. It
// returns the setting previously in effect; when err is non-nil the returned
// prev is meaningless — use OFDLockingEnabled for the current state.
//
// A POSIX record lock belongs to the (process, inode) pair: the kernel drops
// every POSIX lock the process holds on a file at any close(2) of any
// descriptor of that file. Innocent code such as
//
//	f, _ := os.Open(dbPath) // hash, back up, inspect, ...
//	f.Close()
//
// anywhere in the process — a third-party library included — therefore
// silently strips SQLite's own transaction locks and leaves the database
// unprotected against other processes. An OFD lock belongs to the open file
// description through which it was placed and survives such a close. OFD
// locking is opt-in and off by default: unless it is enabled, nothing about
// locking changes.
//
// The switch is process-wide, which is why it is not a DSN parameter: POSIX
// and OFD locks taken by one process are different owners to the kernel and
// conflict with each other, so every connection to a given database file
// inside one process must use the same kind.
//
// OFDLocking must be called before the first database file is opened. From the
// process's first lock attempt on, the mode is fixed and calls attempting to
// change it return ErrOFDLockingTooLate; querying with OFDLockingEnabled, and
// calling OFDLocking with the value already in effect, always work. The call
// overrides the MODERNC_SQLITE_OFD_LOCK environment variable, which enables
// OFD locking when set to anything but the empty string or a value starting
// with "0" and which the library reads once, at initialization time.
//
// OFD locks exist on Linux only; everywhere else OFDLocking returns
// ErrOFDLockingUnavailable. On Linux kernels older than 3.15, and on
// filesystems that reject OFD locks, the first lock attempt fails with EINVAL
// and the library permanently falls back to POSIX locks: OFDLocking returns
// ErrOFDLockingUnavailable from then on and OFDLockingEnabled reports false,
// which is how a caller can detect the fallback.
//
// Enabling OFD locking covers the locks on the database file itself; the locks
// coordinating WAL mode through the -shm file remain POSIX locks.
//
// OFDLocking is safe for concurrent use.
func OFDLocking(on bool) (prev bool, err error) {
	return msqlite.OFDLocking(on)
}

// OFDLockingEnabled reports whether Linux Open File Description locks are in
// effect for database files in this process; see OFDLocking. It reports false
// where OFD locks are unavailable: on every platform but Linux, and on Linux
// once the kernel has rejected them and the library has fallen back to POSIX
// locks.
func OFDLockingEnabled() bool {
	return msqlite.OFDLockingEnabled()
}
