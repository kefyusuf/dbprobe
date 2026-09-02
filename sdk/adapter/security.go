package adapter

type Privilege struct {
	Name   string `json:"name"`
	Scope  string `json:"scope,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SecurityProfile struct {
	ReadOnlyGuaranteed bool        `json:"read_only_guaranteed"`
	Required           []Privilege `json:"required,omitempty"`
	Recommended        []Privilege `json:"recommended,omitempty"`
	Missing            []Privilege `json:"missing,omitempty"`
}
