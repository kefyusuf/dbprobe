package capability

import "sort"

type Capability string

type Set struct{ values map[Capability]struct{} }

func New(values ...Capability) Set {
	set := Set{values: make(map[Capability]struct{}, len(values))}
	for _, value := range values {
		set.values[value] = struct{}{}
	}
	return set
}

func (s Set) Has(value Capability) bool {
	_, ok := s.values[value]
	return ok
}

func (s Set) HasAll(values []Capability) bool {
	for _, value := range values {
		if !s.Has(value) {
			return false
		}
	}
	return true
}

func (s Set) List() []Capability {
	out := make([]Capability, 0, len(s.values))
	for value := range s.values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
