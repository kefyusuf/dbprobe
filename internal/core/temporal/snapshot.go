package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const SnapshotSchemaVersion = "dbprobe.snapshot/v1alpha1"

type SnapshotInput struct {
	TargetFingerprint string
	Engine            string
	AdapterID         string
	AdapterVersion    string
	CollectedAt       time.Time
	Capabilities      []capability.Capability
	Observations      []signal.Observation
	Deltas            []signal.Delta
	Findings          []finding.Finding
}

type Snapshot struct {
	SchemaVersion     string                  `json:"schema_version"`
	ID                string                  `json:"id"`
	TargetFingerprint string                  `json:"target_fingerprint"`
	Engine            string                  `json:"engine"`
	AdapterID         string                  `json:"adapter_id"`
	AdapterVersion    string                  `json:"adapter_version"`
	CollectedAt       time.Time               `json:"collected_at"`
	Capabilities      []capability.Capability `json:"capabilities"`
	Observations      []signal.Observation    `json:"observations"`
	Deltas            []signal.Delta          `json:"deltas"`
	Findings          []finding.Finding       `json:"findings"`
}

func NewSnapshot(in SnapshotInput) (Snapshot, error) {
	if in.TargetFingerprint == "" {
		return Snapshot{}, fmt.Errorf("snapshot target fingerprint is required")
	}
	if in.Engine == "" {
		return Snapshot{}, fmt.Errorf("snapshot engine is required")
	}
	if in.AdapterID == "" {
		return Snapshot{}, fmt.Errorf("snapshot adapter ID is required")
	}
	if in.CollectedAt.IsZero() {
		return Snapshot{}, fmt.Errorf("snapshot collection time is required")
	}

	collectedAt := in.CollectedAt.UTC()
	seed := in.TargetFingerprint + "|" + collectedAt.Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(seed))

	return Snapshot{
		SchemaVersion:     SnapshotSchemaVersion,
		ID:                hex.EncodeToString(sum[:16]),
		TargetFingerprint: in.TargetFingerprint,
		Engine:            in.Engine,
		AdapterID:         in.AdapterID,
		AdapterVersion:    in.AdapterVersion,
		CollectedAt:       collectedAt,
		Capabilities:      normalizeCapabilities(in.Capabilities),
		Observations:      cloneObservations(in.Observations),
		Deltas:            append([]signal.Delta(nil), in.Deltas...),
		Findings:          cloneFindings(in.Findings),
	}, nil
}

func normalizeCapabilities(values []capability.Capability) []capability.Capability {
	seen := make(map[capability.Capability]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	out := make([]capability.Capability, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneObservations(values []signal.Observation) []signal.Observation {
	if values == nil {
		return []signal.Observation{}
	}
	out := make([]signal.Observation, len(values))
	for i, value := range values {
		out[i] = cloneObservation(value)
	}
	return out
}

func cloneObservation(value signal.Observation) signal.Observation {
	out := value
	if value.Number != nil {
		n := *value.Number
		out.Number = &n
	}
	if value.Text != nil {
		t := *value.Text
		out.Text = &t
	}
	if value.Boolean != nil {
		b := *value.Boolean
		out.Boolean = &b
	}
	return out
}

func cloneFindings(values []finding.Finding) []finding.Finding {
	if values == nil {
		return []finding.Finding{}
	}
	out := make([]finding.Finding, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Evidence = cloneObservations(value.Evidence)
	}
	return out
}
