package contracheque

import (
	"bytes"
	"image/png"
)

type dims struct{ w, h int }

func pngSize(data []byte) (dims, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	return dims{cfg.Width, cfg.Height}, err
}
