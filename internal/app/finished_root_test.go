package app

import (
	"reflect"
	"slices"
	"testing"
)

func TestFinishedEventFieldOrder(t *testing.T) {
	typ := reflect.TypeFor[FinishedEvent]()
	var got []string
	for field := range typ.Fields() {
		got = append(got, field.Name)
	}
	if want := []string{"Status", "Error", "Root", "RootID", "Summary"}; !slices.Equal(got, want) {
		t.Fatalf("FinishedEvent fields = %v, want %v", got, want)
	}
}

func TestFinishedEventRootIsScanRoot(t *testing.T) {
	_, rt, root, _ := scanDone(t, Deps{})
	evs := rt.eventsNamed(EventScanFinished)
	if len(evs) != 1 {
		t.Fatalf("scan:finished emitted %d times, want 1", len(evs))
	}
	fe, ok := evs[0].Data.(FinishedEvent)
	if !ok || fe.Root != root {
		t.Fatalf("scan:finished data = %#v, want Root %q", evs[0].Data, root)
	}
}
