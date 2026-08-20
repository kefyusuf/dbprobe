package fake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type fakeAdapter struct{}

func New() adapter.Adapter { return fakeAdapter{} }

func (fakeAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{
		ID:              "fake",
		Name:            "Fake",
		Version:         "0.1.0",
		ContractVersion: adapter.ContractVersion,
	}
}

func (fakeAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "fake" }

func (fakeAdapter) Open(_ context.Context, spec adapter.TargetSpec, _ adapter.OpenOptions) (adapter.Runtime, error) {
	if spec.Scheme != "fake" {
		return nil, fmt.Errorf("fake adapter does not support scheme %q", spec.Scheme)
	}
	u, err := url.Parse(spec.RawURL)
	if err != nil || u.Host != "local" {
		return nil, fmt.Errorf("fake adapter requires target host local")
	}
	sum := sha256.Sum256([]byte("fake|local"))
	return newRuntime(adapter.TargetMetadata{
		Engine:      "fake",
		AdapterID:   "fake",
		Fingerprint: hex.EncodeToString(sum[:8]),
		DisplayName: "local",
	}), nil
}
