package mysql

import (
	"context"
	"fmt"

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

func (mysqlAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return nil, fmt.Errorf("MySQL runtime is not initialized")
}
