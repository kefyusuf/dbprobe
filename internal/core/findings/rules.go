package findings

import "github.com/kefyusuf/dbprobe/sdk/finding"

func Rules() []finding.Rule {
	return []finding.Rule{connectionSaturationRule{}}
}
