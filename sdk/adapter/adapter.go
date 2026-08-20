package adapter

import (
	"context"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

const ContractVersion = "v0.1"

type Metadata struct {
	ID              string
	Name            string
	Version         string
	ContractVersion string
}
type OpenOptions struct{}

type Adapter interface {
	Metadata() Metadata
	Match(TargetSpec) bool
	Open(context.Context, TargetSpec, OpenOptions) (Runtime, error)
}

type Runtime interface {
	Target() TargetMetadata
	Capabilities() capability.Set
	Collectors() []collector.Collector
	Rules() []finding.Rule
	SecurityProfile() SecurityProfile
	Close() error
}
