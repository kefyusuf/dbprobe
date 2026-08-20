package finding

import (
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type ID string
type Severity string

type Finding struct {
	ID         ID                   `json:"id"`
	Title      string               `json:"title"`
	Severity   Severity             `json:"severity"`
	Object     object.Ref           `json:"object"`
	Evidence   []signal.Observation `json:"evidence,omitempty"`
	Summary    string               `json:"summary"`
	Guidance   string               `json:"guidance,omitempty"`
	Confidence float64              `json:"confidence"`
}

type AnalysisContext struct {
	Capabilities capability.Set
	Current      []signal.Observation
	Previous     []signal.Observation
	Deltas       []signal.Delta
}

type Rule interface {
	ID() ID
	Requires() []capability.Capability
	Evaluate(AnalysisContext) []Finding
}
