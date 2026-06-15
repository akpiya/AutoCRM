package common_test

import (
	"testing"
	"time"

	"github.com/akpiya/autocrm/internal/common"
	"github.com/akpiya/autocrm/internal/testfixtures"
)

func TestAppleNsToUnix(t *testing.T) {
	if _, ok := common.AppleNsToUnix(0); ok {
		t.Fatal("expected false for 0")
	}
	ns := int64(testfixtures.AppleNS20010102 + 1_000_000_000)
	got, ok := common.AppleNsToUnix(ns)
	if !ok {
		t.Fatal("expected true")
	}
	expected := float64(common.AppleEpochUTC.Unix()) + float64(common.AppleEpochUTC.Nanosecond())/1e9 + float64(ns)/1e9
	if got != expected {
		t.Fatalf("got %v want %v", got, expected)
	}
	_ = time.UTC
}
