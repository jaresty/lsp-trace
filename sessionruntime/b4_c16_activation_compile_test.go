package sessionruntime

import (
	"errors"
	"testing"
	"unsafe"
)

const (
	frozenPrivateB4ReservationSizeC16Guard      = uintptr(41664)
	frozenPrivateB4ReservationAlignC16Guard     = uintptr(8)
	frozenPrivateB4ReservationTokenC16Guard     = uintptr(0)
	frozenPrivateB4ReservationSelectionC16Guard = uintptr(32)
	frozenPrivateB4ReservationExplicitC16Guard  = uintptr(488)
	frozenPrivateB4ReservationResponsesC16Guard = uintptr(41656)
)

func TestPrivateB4C16ActivationCompileContract(t *testing.T) {
	if unsafe.Sizeof(privateB4Reservation{}) != frozenPrivateB4ReservationSizeC16Guard || unsafe.Alignof(privateB4Reservation{}) != frozenPrivateB4ReservationAlignC16Guard || unsafe.Offsetof(privateB4Reservation{}.token) != frozenPrivateB4ReservationTokenC16Guard || unsafe.Offsetof(privateB4Reservation{}.selection) != frozenPrivateB4ReservationSelectionC16Guard || unsafe.Offsetof(privateB4Reservation{}.explicitBytes) != frozenPrivateB4ReservationExplicitC16Guard || unsafe.Offsetof(privateB4Reservation{}.retainedResponses) != frozenPrivateB4ReservationResponsesC16Guard {
		t.Fatal("ASSERT_C16_LEGACY_LAYOUT_CHANGED")
	}
	var req RoundTripRequest
	if req.EnablePrivateC16ObjectAccounting {
		t.Fatal("ASSERT_C16_MARKER_DEFAULT_OFF")
	}
	var enabled *privateB4ReservationC16
	if enabled != nil {
		t.Fatal("ASSERT_C16_DISTINCT_REPRESENTATION")
	}
	called := false
	err := (PrivateB4DefinitionBorrow{}).WithObjectAdmission(func() error { called = true; return nil })
	if !errors.Is(err, ErrPrivateB4C16NotEnabled) || called {
		t.Fatalf("ASSERT_C16_NOT_ENABLED err=%v called=%t", err, called)
	}
}
