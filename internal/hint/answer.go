package hint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// Answer is what a model said: the box round the first mistake, in pixels of the crop, or nil when
// it found none; the trap of the task the mistake falls into, or "" for none; and the hint the
// learner reads. Whether the hint keeps the rules of its form and leaks nothing is T75's to check.
type Answer struct {
	Box  *judge.Box `json:"box"`
	Trap string     `json:"trap"`
	Hint string     `json:"hint"`
}

// Read takes a model's answer apart as strictly as the schema: the box, the trap and the hint, all
// three and nothing else, the box null or four corners, put in order, and the trap null or the ID
// of one of the task's traps. The answer as the model gave it is the run's to keep.
func Read(answer string, traps []sample.Trap) (*Answer, error) {
	var raw struct {
		Box  json.RawMessage `json:"box"`
		Trap json.RawMessage `json:"trap"`
		Hint *string         `json:"hint"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(answer)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("hint: the answer is off the schema: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("hint: the answer has more after it")
	}
	if raw.Box == nil || raw.Trap == nil || raw.Hint == nil {
		return nil, errors.New("hint: the answer lacks a field")
	}
	a := &Answer{Hint: *raw.Hint}
	if string(raw.Box) != "null" {
		box, err := judge.ReadBox(raw.Box)
		if err != nil {
			return nil, fmt.Errorf("hint: %w", err)
		}
		a.Box = box
	}
	if string(raw.Trap) != "null" {
		if err := json.Unmarshal(raw.Trap, &a.Trap); err != nil {
			return nil, fmt.Errorf("hint: the trap is off the schema: %w", err)
		}
		if !slices.ContainsFunc(traps, func(t sample.Trap) bool { return t.ID == a.Trap }) {
			return nil, fmt.Errorf("hint: the trap %q is none of the task's", a.Trap)
		}
	}
	return a, nil
}

// Place is where a box in pixels of a crop stands on it, as the adapter of the provider tells it
// (provider.Place; R53 spec): in whole thousandths of the crop's height and width, [ymin, xmin,
// ymax, xmax], 0 to 1000. A box that has no area on the picture has no place.
func Place(b judge.Box, c *render.Crop) ([4]int64, bool) {
	return provider.Place(b.X1, b.Y1, b.X2, b.Y2, c.Width, c.Height)
}

// OnSheet is where a place on a crop, in thousandths, stands on the sheet, in whole units, as the
// device would tell it by the crop's place on the sheet (R53 spec): the box judge.Hit takes.
func OnSheet(p [4]int64, c *render.Crop) judge.Box {
	at := func(t int64, lo, hi float64) float64 { return lo + float64(t)*(hi-lo)/1000 }
	b := c.SheetBox
	return judge.Box{
		X1: int(math.Floor(at(p[1], b[1], b[3]))), Y1: int(math.Floor(at(p[0], b[0], b[2]))),
		X2: int(math.Ceil(at(p[3], b[1], b[3]))), Y2: int(math.Ceil(at(p[2], b[0], b[2]))),
	}
}

// OnCrop is a box in units of the sheet as it stands on a crop, in its pixels: the box a model that
// is always right gives for a mistake there.
func OnCrop(b judge.Box, c *render.Crop) judge.Box {
	x := func(u int) float64 { return (float64(u) - c.SheetBox[1]) * c.Scale }
	y := func(u int) float64 { return (float64(u) - c.SheetBox[0]) * c.Scale }
	return judge.Box{
		X1: int(math.Floor(x(b.X1))), Y1: int(math.Floor(y(b.Y1))),
		X2: int(math.Ceil(x(b.X2))), Y2: int(math.Ceil(y(b.Y2))),
	}
}

// Hit is the part of a sheet's markup a box in pixels of its crop hits, by its number, from 1, as
// the judge's box hits it (judge.Hit), and the box's place on the crop: 0 and no place when the box
// has no area on the picture.
func Hit(s *sample.Sample, b judge.Box, c *render.Crop) (int, [4]int64, bool) {
	p, ok := Place(b, c)
	if !ok {
		return 0, p, false
	}
	return judge.Hit(s, OnSheet(p, c)), p, true
}
