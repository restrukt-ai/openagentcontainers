package discovery

import (
	"reflect"
	"testing"
)

func TestHoistLatestPresent(t *testing.T) {
	t.Parallel()

	in := []string{"v1.0", "v2.0", "latest", "stable"}
	want := []string{"latest", "v1.0", "v2.0", "stable"}

	got := hoistLatest(in)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHoistLatestAlreadyFirst(t *testing.T) {
	t.Parallel()

	in := []string{"latest", "v1.0", "v2.0"}
	want := []string{"latest", "v1.0", "v2.0"}

	got := hoistLatest(in)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHoistLatestAbsent(t *testing.T) {
	t.Parallel()

	in := []string{"v1.0", "stable", "v2.0"}

	got := hoistLatest(in)

	if !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v, want %v (unchanged)", got, in)
	}
}

func TestHoistLatestEmpty(t *testing.T) {
	t.Parallel()

	got := hoistLatest(nil)

	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestHoistLatestSingleLatest(t *testing.T) {
	t.Parallel()

	in := []string{"latest"}
	got := hoistLatest(in)

	if !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v, want %v", got, in)
	}
}

func TestHoistLatestLastPosition(t *testing.T) {
	t.Parallel()

	in := []string{"v1.0", "v2.0", "latest"}
	want := []string{"latest", "v1.0", "v2.0"}

	got := hoistLatest(in)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHoistLatestDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	in := []string{"v1.0", "latest", "v2.0"}
	orig := []string{"v1.0", "latest", "v2.0"}

	hoistLatest(in)

	if !reflect.DeepEqual(in, orig) {
		t.Fatalf("input was mutated: got %v, want %v", in, orig)
	}
}
