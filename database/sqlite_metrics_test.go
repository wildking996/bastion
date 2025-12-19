package database

import (
	"errors"
	"testing"
)

func TestClassifySQLiteError_Busy(t *testing.T) {
	busy, locked := classifySQLiteError(errors.New("SQLITE_BUSY: database is locked"))
	if !busy || locked {
		t.Fatalf("expected busy=true locked=false, got busy=%v locked=%v", busy, locked)
	}
}

func TestClassifySQLiteError_Locked(t *testing.T) {
	busy, locked := classifySQLiteError(errors.New("SQLITE_LOCKED: database table is locked"))
	if busy || !locked {
		t.Fatalf("expected busy=false locked=true, got busy=%v locked=%v", busy, locked)
	}
}
