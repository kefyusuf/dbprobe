package signal

import (
	"time"

	"github.com/kefyusuf/dbprobe/sdk/object"
)

type Key string
type Unit string
type Exactness string
type Sensitivity string

const (
	UnitCount        Unit = "count"
	UnitMilliseconds Unit = "milliseconds"
	UnitBytes        Unit = "bytes"
	UnitRatio        Unit = "ratio"
	UnitSeconds      Unit = "seconds"

	ExactnessScraped     Exactness = "scraped"
	ExactnessCumulative  Exactness = "cumulative"
	ExactnessSampled     Exactness = "sampled"
	ExactnessEstimated   Exactness = "estimated"
	ExactnessUnavailable Exactness = "unavailable"
	ExactnessReset       Exactness = "reset"

	SensitivityMetadata   Sensitivity = "metadata"
	SensitivityQueryShape Sensitivity = "query_shape"
	SensitivityQueryText  Sensitivity = "query_text"
)

type Observation struct {
	Key         Key         `json:"key"`
	Object      object.Ref  `json:"object"`
	Unit        Unit        `json:"unit"`
	Exactness   Exactness   `json:"exactness"`
	Number      *float64    `json:"number,omitempty"`
	Text        *string     `json:"text,omitempty"`
	Boolean     *bool       `json:"boolean,omitempty"`
	CollectedAt time.Time   `json:"collected_at"`
	Sensitivity Sensitivity `json:"sensitivity"`
	Source      string      `json:"source,omitempty"`
	Reason      string      `json:"reason,omitempty"`
}

func NumberObservation(key Key, ref object.Ref, value float64, unit Unit, exactness Exactness, sensitivity Sensitivity, at time.Time) Observation {
	return Observation{Key: key, Object: ref, Unit: unit, Exactness: exactness, Number: &value, CollectedAt: at, Sensitivity: sensitivity}
}

func (o Observation) Numeric() (float64, bool) {
	if o.Number == nil {
		return 0, false
	}
	return *o.Number, true
}
