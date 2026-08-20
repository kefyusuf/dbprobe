package fake

import (
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

type runtime struct {
	target     adapter.TargetMetadata
	collectors []collector.Collector
}

func newRuntime(target adapter.TargetMetadata) *runtime {
	return &runtime{target: target, collectors: []collector.Collector{healthCollector{}, &workloadCollector{}}}
}

func (r *runtime) Target() adapter.TargetMetadata { return r.target }
func (r *runtime) Capabilities() capability.Set { return capability.New("activity.sessions", "workload.query_summary") }
func (r *runtime) Collectors() []collector.Collector { return append([]collector.Collector(nil), r.collectors...) }
func (r *runtime) Rules() []finding.Rule { return []finding.Rule{} }
func (r *runtime) SecurityProfile() adapter.SecurityProfile { return adapter.SecurityProfile{ReadOnlyGuaranteed: true} }
func (r *runtime) Close() error { return nil }
