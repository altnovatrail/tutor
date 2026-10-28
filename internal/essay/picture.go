// Package essay is the assessment of an essay by the module author's rubric in the measurement of
// epic E4 (T82; docs/architecture.md, 3.4, «Сочинения»): what a model sees of a synthetic essay.
package essay

import (
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// Picture is an essay as a model sees it: the lines it is written in, the crop of its page and the
// crop as PNG.
type Picture struct {
	Lines []string
	Crop  *render.Crop
	PNG   []byte
}

// PictureOf draws an essay from the seed (synth.Essay) and cuts the crop of its page by the rule of
// the eval protocol, with its digits digit px high, or less when the crop would be too large
// (render.CropOf).
func PictureOf(e *sample.Essay, seed uint64, digit int) (*Picture, error) {
	lines, strokes := synth.Essay(e, seed)
	c, err := render.CropOf(strokes, nil, digit)
	if err != nil {
		return nil, err
	}
	img, err := c.Draw(strokes)
	if err != nil {
		return nil, err
	}
	png, err := render.PNG(img)
	if err != nil {
		return nil, err
	}
	return &Picture{Lines: lines, Crop: c, PNG: png}, nil
}
