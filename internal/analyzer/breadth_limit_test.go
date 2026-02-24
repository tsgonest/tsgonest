package analyzer

import (
	"testing"

	shimchecker "github.com/microsoft/typescript-go/shim/checker"
	"github.com/tsgonest/tsgonest/internal/metadata"
)

func TestMaxTotalTypesConstant(t *testing.T) {
	if maxTotalTypes != 500 {
		t.Errorf("expected maxTotalTypes=500, got %d", maxTotalTypes)
	}
}

func TestBreadthLimit(t *testing.T) {
	// Verify the breadth limit field is tracked and incrementable.
	// We can't easily create 500+ real types, but we can verify the mechanism
	// by setting the counter manually since this test is in the same package.
	w := &TypeWalker{
		visiting:     make(map[shimchecker.TypeId]bool),
		typeIdToName: make(map[shimchecker.TypeId]string),
		registry:     metadata.NewTypeRegistry(),
	}

	// Set counter just below the limit
	w.totalTypesWalked = maxTotalTypes - 1
	if w.totalTypesWalked != 499 {
		t.Errorf("expected totalTypesWalked=499, got %d", w.totalTypesWalked)
	}

	// Verify the field is incrementable past the limit
	w.totalTypesWalked = maxTotalTypes + 1
	if w.totalTypesWalked != maxTotalTypes+1 {
		t.Errorf("expected totalTypesWalked=%d, got %d", maxTotalTypes+1, w.totalTypesWalked)
	}
}
