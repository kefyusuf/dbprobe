package main

import (
	"github.com/kefyusuf/dbprobe/adapters/fake"
	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
)

func newAdapterRegistry() (*adapterregistry.Registry, error) {
	return adapterregistry.New(fake.New(), mysqladapter.New())
}
