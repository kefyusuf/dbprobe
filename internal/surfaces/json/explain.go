package json

import (
	"encoding/json"
	"io"

	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
)

func RenderExplain(w io.Writer, report appexplain.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
