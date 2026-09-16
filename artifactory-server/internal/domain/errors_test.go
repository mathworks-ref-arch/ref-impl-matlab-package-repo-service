// Copyright 2026 The MathWorks, Inc.

package domain

import (
	"errors"
	"testing"
)

func TestSentinelErrors_Exist(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	if ErrConflict == nil {
		t.Fatal("ErrConflict should not be nil")
	}
	if ErrValidation == nil {
		t.Fatal("ErrValidation should not be nil")
	}
}

func TestSentinelErrors_Distinguishable(t *testing.T) {
	if errors.Is(ErrNotFound, ErrConflict) {
		t.Fatal("ErrNotFound should not match ErrConflict")
	}
	if errors.Is(ErrNotFound, ErrValidation) {
		t.Fatal("ErrNotFound should not match ErrValidation")
	}
}
