package capability_test

import (
	"reflect"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestSetHasAndListsUniqueValuesInStableOrder(t *testing.T) {
	set := capability.New("storage.cache", "activity.sessions", "storage.cache")
	if !set.Has("activity.sessions") {
		t.Fatal("expected activity.sessions")
	}
	want := []capability.Capability{"activity.sessions", "storage.cache"}
	if got := set.List(); !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
}
