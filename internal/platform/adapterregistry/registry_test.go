package adapterregistry_test

import (
	"context"
	"testing"

	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type stubAdapter struct{ id, scheme, contract string }

func (s stubAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: s.id, Name: s.id, Version: "test", ContractVersion: s.contract}
}
func (s stubAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == s.scheme }
func (s stubAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return nil, nil
}

func TestResolveReturnsMatchingAdapter(t *testing.T) {
	r, err := adapterregistry.New(stubAdapter{"fake", "fake", adapter.ContractVersion})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := adapter.ParseTarget("fake://local")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Resolve(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata().ID != "fake" {
		t.Fatalf("got %q", got.Metadata().ID)
	}
}

func TestResolveFailsWhenNoAdapterMatches(t *testing.T) {
	r, err := adapterregistry.New(stubAdapter{"fake", "fake", adapter.ContractVersion})
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := adapter.ParseTarget("redis://local")
	if _, err := r.Resolve(spec); err == nil {
		t.Fatal("expected no-match error")
	}
}

func TestNewRejectsDuplicateIDs(t *testing.T) {
	_, err := adapterregistry.New(
		stubAdapter{"same", "a", adapter.ContractVersion},
		stubAdapter{"same", "b", adapter.ContractVersion},
	)
	if err == nil {
		t.Fatal("expected duplicate-ID error")
	}
}

func TestNewRejectsContractMismatch(t *testing.T) {
	_, err := adapterregistry.New(stubAdapter{"fake", "fake", "v999"})
	if err == nil {
		t.Fatal("expected contract-version error")
	}
}
