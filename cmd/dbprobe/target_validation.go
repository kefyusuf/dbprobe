package main

import (
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func validateTarget(registry *adapterregistry.Registry, rawTarget string) error {
	spec, err := adapter.ParseTarget(rawTarget)
	if err != nil {
		return err
	}
	_, err = registry.Resolve(spec)
	return err
}
