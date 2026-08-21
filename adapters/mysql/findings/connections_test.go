package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestConnectionSaturationWarnsAtEightyFivePercentOnly(t *testing.T) {
	ctx := finding.AnalysisContext{Capabilities: capability.New("mysql.performance_schema"), Current: []signal.Observation{
		number("core.connections.used", "mysql.instance", "db", 85),
		number("core.connections.limit", "mysql.instance", "db", 100),
	}}
	assertFinding(t, connectionSaturationRule{}.Evaluate(ctx), "core.connection_saturation", "warn")

	ctx.Current[0] = number("core.connections.used", "mysql.instance", "db", 84)
	if got := (connectionSaturationRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("connection saturation fired below 85%%: %#v", got)
	}
}
