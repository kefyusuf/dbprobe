package jsonsurface

import (
	"encoding/json"
	"io"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
)

func Render(w io.Writer, report inspect.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
