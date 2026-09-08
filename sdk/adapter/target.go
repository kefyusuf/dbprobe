package adapter

import (
	"fmt"
	"net/url"
)

type TargetSpec struct {
	RawURL string
	Scheme string
}

type TargetMetadata struct {
	Engine      string `json:"engine"`
	AdapterID   string `json:"adapter_id"`
	Fingerprint string `json:"fingerprint"`
	DisplayName string `json:"display_name"`
}

func ParseTarget(raw string) (TargetSpec, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return TargetSpec{}, fmt.Errorf("invalid target URL")
	}
	return TargetSpec{RawURL: raw, Scheme: u.Scheme}, nil
}
