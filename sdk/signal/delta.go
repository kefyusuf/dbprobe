package signal

import "github.com/kefyusuf/dbprobe/sdk/object"

type Delta struct {
	Key           Key        `json:"key"`
	Object        object.Ref `json:"object"`
	Unit          Unit       `json:"unit"`
	Delta         float64    `json:"delta"`
	RatePerSecond float64    `json:"rate_per_second"`
	WindowSeconds float64    `json:"window_seconds"`
	Exactness     Exactness  `json:"exactness"`
}
