package mysql

import (
	"context"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type mysqlAdapter struct{}

func New() adapter.Adapter { return mysqlAdapter{} }

func (mysqlAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{
		ID:              "mysql",
		Name:            "MySQL",
		Version:         "0.1.0",
		ContractVersion: adapter.ContractVersion,
	}
}

func (mysqlAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "mysql" }

func (mysqlAdapter) Open(ctx context.Context, spec adapter.TargetSpec, _ adapter.OpenOptions) (adapter.Runtime, error) {
	cfg, err := ParseConfig(spec.RawURL)
	if err != nil {
		return nil, err
	}
	runtime, err := openRuntime(ctx, cfg)
	if err != nil {
		return nil, sanitizeError(err, cfg)
	}
	return runtime, nil
}
