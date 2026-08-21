package adapter

import "context"

type ExplainRequest struct {
	Statement string `json:"statement"`
}

type ExplainResult struct {
	Engine    string `json:"engine"`
	Format    string `json:"format"`
	Estimated bool   `json:"estimated"`
	Plan      []byte `json:"plan"`
}

type PlanExplainer interface {
	ExplainPlan(context.Context, ExplainRequest) (ExplainResult, error)
}
