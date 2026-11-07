package render

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/contracts"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// cropVectors is spec's test vectors of the crop, contracts.Vectors eval/v1/crops.json
// (contracts/README.md of spec, section 12; Д96 in spec's RUN.md): canvas's cases (R25 canvas) in
// the form of the protocol, their strokes a final geometry and their crops a CropSet without PNG.
// They hold the crop's numbers, not its pixels. Their messages are read by protojson.
type cropVectors struct {
	Rule struct {
		Margin  int   `json:"margin"`
		Flat    int   `json:"flat"`
		Ceiling int   `json:"ceiling"`
		Side    int   `json:"side"`
		Edge    int   `json:"edge"`
		Tokens  int   `json:"tokens"`
		Patch   int   `json:"patch"`
		Heights []int `json:"heights"`
	} `json:"rule"`
	Cases []cropCase `json:"cases"`
}

// cropCase is a case of the vectors as the file writes it.
type cropCase struct {
	Name        string            `json:"name"`
	DigitHeight int               `json:"digit_height"`
	Strokes     []json.RawMessage `json:"strokes"`
	Groups      []json.RawMessage `json:"groups"`
	Bundles     [][]int           `json:"bundles"`
	Crops       json.RawMessage   `json:"crops"`
}

// vectorCase is a case of the vectors as the renderer takes it: its strokes, their IDs as text, its
// groups and its crops.
type vectorCase struct {
	strokes []sample.Stroke
	groups  []lines.StrokeGroup
	crops   *evalv1.CropSet
}

func readCropVectors(t *testing.T) (cropVectors, []vectorCase) {
	t.Helper()
	data, err := fs.ReadFile(contracts.Vectors, "eval/v1/crops.json")
	if err != nil {
		t.Fatal(err)
	}
	var v cropVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	read := func(raw json.RawMessage, m proto.Message) {
		if err := protojson.Unmarshal(raw, m); err != nil {
			t.Fatal(err)
		}
	}
	var cases []vectorCase
	for _, c := range v.Cases {
		vc := vectorCase{crops: &evalv1.CropSet{}}
		var final []*evalv1.FinalStroke
		for _, raw := range c.Strokes {
			st := &evalv1.FinalStroke{}
			read(raw, st)
			final = append(final, st)
		}
		vc.strokes = protocol.Final(final)
		for _, raw := range c.Groups {
			var g evalv1.Group
			read(raw, &g)
			sg := lines.StrokeGroup{Label: g.GetLabel()}
			for _, id := range g.GetStrokeIds() {
				sg.Strokes = append(sg.Strokes, protocol.StrokeID(id))
			}
			vc.groups = append(vc.groups, sg)
		}
		read(c.Crops, vc.crops)
		cases = append(cases, vc)
	}
	return v, cases
}

// The crop's rule is spec's: its numbers, and every number of every case of spec's vectors at each
// height of digits, to the bit — the bundles, the box of the ink and h_med in tenths, the heights
// of the set, the height drawn, the scale, the line, the size, the margins, where the crop lies on
// the sheet and the frames of the groups. h_med is the set's bundle_median as it is, while the
// scale takes it no higher than the ceiling.
func TestCropVectors(t *testing.T) {
	v, cases := readCropVectors(t)
	r := v.Rule
	if got := []int{r.Margin, r.Flat, r.Ceiling, r.Side, r.Edge, r.Tokens, r.Patch}; !slices.Equal(got, []int{CropMargin, cropFlat, cropCeiling, CropSide, cropEdge, cropTokens, 28}) {
		t.Errorf("the vectors' rule is %v", got)
	}
	if !slices.Equal(r.Heights, Heights()) {
		t.Errorf("the vectors' heights are %v, and the measurement's %v", r.Heights, Heights())
	}
	if len(v.Cases) != 34 {
		t.Errorf("%d cases, want spec's 34", len(v.Cases))
	}
	for n, c := range v.Cases {
		vc := cases[n]
		bundles, err := Bundles(vc.strokes)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		var ids [][]int
		for _, b := range bundles {
			var bundle []int
			for _, i := range b {
				id, _ := strconv.Atoi(vc.strokes[i].ID)
				bundle = append(bundle, id)
			}
			slices.Sort(bundle)
			ids = append(ids, bundle)
		}
		slices.SortFunc(ids, func(a, b []int) int { return a[0] - b[0] })
		if !reflect.DeepEqual(ids, c.Bundles) && (len(ids) > 0 || len(c.Bundles) > 0) {
			t.Errorf("%s: bundles %v, want %v", c.Name, ids, c.Bundles)
		}
		es, err := extentsOf(vc.strokes)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if m := medianTenths(es); m != int64(vc.crops.GetBundleMedian()) {
			t.Errorf("%s: h_med %d in tenths, want %d", c.Name, m, vc.crops.GetBundleMedian())
		}
		heights := append(Heights(), c.DigitHeight)
		slices.Sort(heights)
		heights = slices.Compact(heights)
		var given []int
		for _, want := range vc.crops.GetCrops() {
			given = append(given, int(want.GetDigitHeight()))
		}
		if !slices.Equal(given, heights) {
			t.Errorf("%s: crops at %v px, want the heights of eval and the module's, %v", c.Name, given, heights)
		}
		for _, want := range vc.crops.GetCrops() {
			got, err := CropOf(vc.strokes, vc.groups, int(want.GetDigitHeight()))
			if err != nil {
				t.Fatalf("%s at %d px: %v", c.Name, want.GetDigitHeight(), err)
			}
			name := fmt.Sprintf("%s at %d px", c.Name, want.GetDigitHeight())
			b := vc.crops.GetInkBox()
			if box := got.InkBox(); box != [4]int64{int64(b.GetYmin()), int64(b.GetXmin()), int64(b.GetYmax()), int64(b.GetXmax())} {
				t.Errorf("%s: box %v in tenths, want %v", name, box, b)
			}
			if m := got.MedianTenths(); m != int64(vc.crops.GetBundleMedian()) {
				t.Errorf("%s: h_med %d in tenths, want %d", name, m, vc.crops.GetBundleMedian())
			}
			if got.unit != min(int64(vc.crops.GetBundleMedian()), 10*cropCeiling) {
				t.Errorf("%s: the scale takes %d tenths, and h_med is %d", name, got.unit, vc.crops.GetBundleMedian())
			}
			margins := [2]int{int(want.GetMarginTop()), int(want.GetMarginLeft())}
			if got.Drawn != int(want.GetDrawnHeight()) || got.Scale != want.GetScale() || got.Line != want.GetLineWidth() ||
				got.Width != int(want.GetWidth()) || got.Height != int(want.GetHeight()) || got.Margins != margins {
				t.Errorf("%s: drawn %d, scale %v, line %v, %d × %d, margins %v; want %d, %v, %v, %d × %d, %v", name,
					got.Drawn, got.Scale, got.Line, got.Width, got.Height, got.Margins,
					want.GetDrawnHeight(), want.GetScale(), want.GetLineWidth(), want.GetWidth(), want.GetHeight(), margins)
			}
			sb := want.GetSheetBox()
			if got.SheetBox != [4]float64{sb.GetYmin(), sb.GetXmin(), sb.GetYmax(), sb.GetXmax()} {
				t.Errorf("%s: on the sheet %v, want %v", name, got.SheetBox, sb)
			}
			var frames []Frame
			for _, f := range want.GetFrames() {
				fb := f.GetBox()
				frames = append(frames, Frame{Label: f.GetLabel(), Box: [4]int64{int64(fb.GetYmin()), int64(fb.GetXmin()), int64(fb.GetYmax()), int64(fb.GetXmax())}})
			}
			if !reflect.DeepEqual(got.Frames, frames) {
				t.Errorf("%s: frames %v, want %v", name, got.Frames, frames)
			}
			if len(want.GetPng()) != 0 {
				t.Errorf("%s: the vectors hold pixels", name)
			}
		}
	}
}

// Every pixel the ink of a group shades lies within the group's frame, on every case of the vectors
// at each height: the frames reach past the points by half the line and the pixel at its edge.
func TestCropFramesHoldTheInk(t *testing.T) {
	_, cases := readCropVectors(t)
	for n, vc := range cases {
		for _, digit := range Heights() {
			c, err := CropOf(vc.strokes, vc.groups, digit)
			if err != nil {
				t.Fatal(err)
			}
			byID := map[string]sample.Stroke{}
			for _, st := range vc.strokes {
				byID[st.ID] = st
			}
			for _, g := range vc.groups {
				var strokes []sample.Stroke
				for _, id := range g.Strokes {
					strokes = append(strokes, byID[id])
				}
				img, err := c.Draw(strokes)
				if err != nil {
					t.Fatal(err)
				}
				i := slices.IndexFunc(c.Frames, func(f Frame) bool { return f.Label == g.Label })
				f := c.Frames[i].Box
				for y := range c.Height {
					for x := range c.Width {
						if img.Pix[y*c.Width+x] == 0xff {
							continue
						}
						if int64(1000*y) < f[0]*int64(c.Height) || int64(1000*x) < f[1]*int64(c.Width) ||
							int64(1000*(y+1)) > f[2]*int64(c.Height) || int64(1000*(x+1)) > f[3]*int64(c.Width) {
							t.Fatalf("case %d at %d px: the ink of %s at (%d, %d) is off its frame %v", n, digit, g.Label, x, y, f)
						}
					}
				}
			}
		}
	}
}

// The line is max(D′, 12) / 12 px wide: across a long level stroke, its ink adds up to the line's
// width, give or take the quarter of a pixel the samples of a pixel are apart.
func TestCropLine(t *testing.T) {
	// A long level line and a tall one that makes the median 36 units, as a digit of the board.
	strokes := []sample.Stroke{
		{ID: "a", Points: []sample.Point{{X: 100, Y: 100}, {X: 400, Y: 100}}},
		{ID: "b", Points: []sample.Point{{X: 500, Y: 60}, {X: 500, Y: 96}}},
	}
	for _, digit := range append(Heights(), 1, 6) {
		c, img, err := Cropped(strokes, digit)
		if err != nil {
			t.Fatal(err)
		}
		if want := float64(max(c.Drawn, 12)) / 12; c.Line != want {
			t.Errorf("at %d px: a line of %v px, want %v", digit, c.Line, want)
		}
		x := c.Margins[1] + int(150*c.Scale) // the middle of the level line, 150 units from the left
		ink := 0.0
		for y := range c.Height {
			ink += float64(0xff-img.Pix[y*c.Width+x]) / 0xff
		}
		if math.Abs(ink-c.Line) > 0.25 {
			t.Errorf("at %d px: the line's ink across it is %.3f px, and the line %.3f", digit, ink, c.Line)
		}
	}
}

// A sheet that shows no strokes has no crop, and a crop takes digits 1 to 256 px high and the
// strokes of one sheet: each with points on it and an ID of its own, each in one group.
func TestCropRefuses(t *testing.T) {
	dot := sample.Stroke{ID: "a", Points: []sample.Point{{X: 10, Y: 10}}}
	group := func(label string, ids ...string) lines.StrokeGroup {
		return lines.StrokeGroup{Label: label, Strokes: ids}
	}
	if _, err := CropOf(nil, nil, Digit); !errors.Is(err, ErrNoInk) {
		t.Errorf("no strokes: %v, want ErrNoInk", err)
	}
	for _, c := range []struct {
		name    string
		strokes []sample.Stroke
		groups  []lines.StrokeGroup
		digit   int
	}{
		{"a digit of 0 px", []sample.Stroke{dot}, nil, 0},
		{"a digit of 257 px", []sample.Stroke{dot}, nil, MaxDigit + 1},
		{"a stroke without points", []sample.Stroke{{ID: "a"}}, nil, Digit},
		{"a stroke twice", []sample.Stroke{dot, dot}, nil, Digit},
		{"a point off the sheet", []sample.Stroke{{ID: "a", Points: []sample.Point{{X: -1, Y: 10}}}}, nil, Digit},
		{"a point of no number", []sample.Stroke{{ID: "a", Points: []sample.Point{{X: math.NaN(), Y: 10}}}}, nil, Digit},
		{"an empty group", []sample.Stroke{dot}, []lines.StrokeGroup{group("A")}, Digit},
		{"a stroke of a group off the sheet", []sample.Stroke{dot}, []lines.StrokeGroup{group("A", "a", "b")}, Digit},
		{"a stroke in two groups", []sample.Stroke{dot}, []lines.StrokeGroup{group("A", "a"), group("B", "a")}, Digit},
		{"a stroke in no group", []sample.Stroke{dot, {ID: "b", Points: dot.Points}}, []lines.StrokeGroup{group("A", "a")}, Digit},
	} {
		if _, err := CropOf(c.strokes, c.groups, c.digit); err == nil || errors.Is(err, ErrNoInk) {
			t.Errorf("%s: %v, want it refused", c.name, err)
		}
	}
	if _, err := (&Crop{Width: 200, Height: 200, Drawn: 32, unit: 360}).Draw([]sample.Stroke{{ID: "a", Points: []sample.Point{{X: MaxSide + 1}}}}); err == nil {
		t.Error("a point off the sheet drawn on a crop: no error")
	}
}

// cropGolden are the SHA-256 of the pixels of the crops of the synthetic sheets of each scenario,
// finished, at each height of digits.
var cropGolden = map[string]string{
	"correct-16":               "cd6f560f98398b8141e6604c010b258f7bf4614450e475337a78e36da93259c9",
	"correct-24":               "6cf3a6780ed6a450e6b1b4f88df7de7e232ae0c0b4a28bef2fd6d90f80e79ed0",
	"correct-32":               "c80dce257ff10d669719c4620c3bba04f1218a23d73e0fe66b8445a7367dff4c",
	"correct-48":               "c0a3b467855495c7a7f3a43aa2baa09f582ce0746c9a13e5bc9949d088c5eef2",
	"error-16":                 "9e55a2ae4a835c4ff12f90b9154b157907582c2de23dbe9a86a33adc58cddce5",
	"error-24":                 "742c91f1d4b9d38892679e435ba1fa6653f02ff8fb18c236c59402b060fc75ca",
	"error-32":                 "3d35dff7feab8c3fb080ad1ad5d93eb83883503bf4562c3588cf3472ed3d0f1d",
	"error-48":                 "eb86f633d04e3748db5ab969580bab4da07e476c8774109cf414d0d60734a9ea",
	"similar-digits-16":        "e7e545efd03477c25a8d0acaca6b815404943ca188cf4ed1ce26134140196599",
	"similar-digits-24":        "20105c2f1a6535c901074fbb90209802b09f155ae54ee7f2ec3971467aefde0f",
	"similar-digits-32":        "405be1850bec1f266077de9963569e5a690481990a3cd275a0efe50a209d9f54",
	"similar-digits-48":        "74c411611b0a8fef907792c2f02fcf2c4236a33cf94ccc7787c2966f43a53d31",
	"illegible-crossed-out-16": "2a61fc48ec868df413c077d2e03e82ef11289f4f6f64b787e8c279e7864a319e",
	"illegible-crossed-out-24": "2a29b199442bc8bcb7a00c7a6d4bf46798b674c485ca30aee158b85521fad9f0",
	"illegible-crossed-out-32": "6984da34bf54e113eec895d2c2624b0f7e40a1dbd814d8ff276f66f6402058a7",
	"illegible-crossed-out-48": "ea1774bfc77ab7e652af8e66ab31f857e31b9bfb41357a0855e307b3ea4e2594",
	"answer-not-last-16":       "8d6027a2e5277726172d441253f48153315eb6ca6bf538c37a324690112b34cc",
	"answer-not-last-24":       "908804f7fcbae44de90cb3a92ef5bd8f63035d14ab3a39744462c38d7ffe5b25",
	"answer-not-last-32":       "8cd8dc0cd7d6322b45080a03736097dec3bfd3d6ef9ebbd9c5d012739fe35a6f",
	"answer-not-last-48":       "619bd8389dc8297d85cb161c954f90e5987203b82c1f2ebdd2fb28dd347afec6",
	"nearly-empty-16":          "70e12e8160366cd29c3ab2796c01d5d24442a44b676d71448a8f02ad685224a6",
	"nearly-empty-24":          "c7c36a080704f182f801601fb442912af138fb197917b48beca36aa2c3fcdafc",
	"nearly-empty-32":          "af2e5c299c8da3d4df6c4bd61461db1ee5e7c57e49c381ff2638332668e1120c",
	"nearly-empty-48":          "cc6073831b84e2572da62922a851223ee921081320ba7364ad8f0f48cfe87448",
	"mirrored-digits-16":       "df8e2f133f1bced3de36a87e928318245caf793e015fe482c2f8e1c11d9b19ba",
	"mirrored-digits-24":       "7762b413a7d979b4f4f95a88e4d730b5f06c595ce077a6a5b4ef3e8de125fc31",
	"mirrored-digits-32":       "cd3656dd9fd3b780ec65d1e124234bca15cf636aa414ce7c06ac4a038265be8c",
	"mirrored-digits-48":       "73b7b80b56b1ba101552270b7af00686b1e0ededcfb7fe163709fece5e4eed49",
	"erasure-fixed-16":         "ce5e6eacfa848a67f314b3e010cdf9cb990d7411b2c3e1255c265d3a2ac3cf72",
	"erasure-fixed-24":         "a9685d431b36ec1660fc20959de35791cb76ace16aac5cc8329f8701a07b192b",
	"erasure-fixed-32":         "5f7c751e7e275981bbceba1ac1f554dea616f8b2dbdcfa5febee4fd04578412a",
	"erasure-fixed-48":         "d609e75e777fc2da7873259472a5db8ab3e03e256bc060877086563ac503454a",
	"erasure-broken-16":        "24a234ed5056f612e010d618520633052107ad1e68a7497777dde64cbc9ea263",
	"erasure-broken-24":        "ece42c16042539ae5f896d27623ec470f35d6b1d9a7f7488e7df560f5fd39872",
	"erasure-broken-32":        "e6403fb98b947e062019118db5250c9ff6ca122a042c03dcb882f6eb4a80f71f",
	"erasure-broken-48":        "91afaed2d9cd12d9ec0b37870b4b91efd25fe2cf31feef5571adcc418a8e60bf",
	"injection-16":             "2177a8af47c739c8fadca3ac7be37bf7fddd98893e1c9373771f3e94bf018e24",
	"injection-24":             "3113da093192995e4ac1b906e2d301c7c8a400b8d08294fb1196d914c3bd2191",
	"injection-32":             "6d1ae51b2f039394cc5d8d6f5762aca8244b8613fbc93a2c82a39a2985765e5d",
	"injection-48":             "123f5e8f0d1df536b41c1bd2982106c767e8eb2777f56ddfdf5f322b6dcbca21",
}

// The crops of the synthetic sheets are drawn the same on every machine: their pixels hash to the
// golden ones, and each crop is the size its numbers give.
func TestCropGolden(t *testing.T) {
	for _, s := range synth.Samples() {
		for _, digit := range Heights() {
			c, img, err := CropFinal(s, digit)
			if err != nil {
				t.Fatalf("%s at %d px: %v", s.Scenario, digit, err)
			}
			if b := img.Bounds(); b.Dx() != c.Width || b.Dy() != c.Height {
				t.Errorf("%s at %d px: %v, want %d × %d", s.Scenario, digit, b, c.Width, c.Height)
			}
			name := fmt.Sprintf("%s-%d", s.Scenario, digit)
			got := fmt.Sprintf("%x", sha256.Sum256(img.Pix))
			if want, has := cropGolden[name]; !has {
				t.Errorf("%q: %q, // no golden crop", name, got)
			} else if got != want {
				t.Errorf("%s: the crop's pixels hash to %s, want %s", name, got, want)
			}
		}
	}
}

// A crop draws other strokes of its sheet by its own numbers too, and what falls off it is cut
// off: a stroke left of the work, at places below zero on the crop, leaves it white.
func TestCropDrawsOtherStrokes(t *testing.T) {
	c, _, err := Cropped([]sample.Stroke{{ID: "a", Points: []sample.Point{{X: 500, Y: 100}, {X: 500, Y: 136}}}}, Digit)
	if err != nil {
		t.Fatal(err)
	}
	img, err := c.Draw([]sample.Stroke{{ID: "b", Points: []sample.Point{{X: 10, Y: 90}, {X: 20, Y: 140}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range img.Pix {
		if v != 0xff {
			t.Fatal("a stroke off the crop left ink on it")
		}
	}
}

// A crop's PNG is as the eval protocol writes one (contracts/README.md of spec, section 11): grey
// of 8 bits, colour type 0, not interlaced, of the crop's size, the chunks IHDR, IDAT and IEND
// alone, its IDAT one zlib stream with nothing after it, each row with a filter from 0 to 4. The
// case is the largest crop of the vectors.
func TestCropPNGChunks(t *testing.T) {
	v, cases := readCropVectors(t)
	n := slices.IndexFunc(v.Cases, func(c cropCase) bool { return c.Name == "limit" })
	if n < 0 {
		t.Fatal("no case limit")
	}
	for _, digit := range Heights() {
		c, err := CropOf(cases[n].strokes, cases[n].groups, digit)
		if err != nil {
			t.Fatal(err)
		}
		data, err := c.PNG(cases[n].strokes)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			t.Fatalf("at %d px: no PNG signature", digit)
		}
		var kinds []string
		var ihdr, idat []byte
		for rest := data[8:]; len(rest) > 0; {
			size := 0
			if len(rest) >= 12 {
				size = int(binary.BigEndian.Uint32(rest))
			}
			if len(rest) < 12 || size > len(rest)-12 {
				t.Fatalf("at %d px: a chunk cut short", digit)
			}
			kind, body := string(rest[4:8]), rest[8:8+size]
			if crc32.ChecksumIEEE(rest[4:8+size]) != binary.BigEndian.Uint32(rest[8+size:]) {
				t.Errorf("at %d px: the CRC of %s is wrong", digit, kind)
			}
			kinds = append(kinds, kind)
			switch kind {
			case "IHDR":
				ihdr = body
			case "IDAT":
				idat = append(idat, body...)
			}
			rest = rest[12+size:]
		}
		if seq := strings.Join(kinds, " "); !regexp.MustCompile(`^IHDR( IDAT)+ IEND$`).MatchString(seq) {
			t.Errorf("at %d px: chunks %s", digit, seq)
		}
		if len(ihdr) != 13 || int(binary.BigEndian.Uint32(ihdr)) != c.Width || int(binary.BigEndian.Uint32(ihdr[4:])) != c.Height ||
			ihdr[8] != 8 || ihdr[9] != 0 || ihdr[10] != 0 || ihdr[11] != 0 || ihdr[12] != 0 {
			t.Errorf("at %d px: IHDR %v for %d × %d", digit, ihdr, c.Width, c.Height)
		}
		r := bytes.NewReader(idat)
		z, err := zlib.NewReader(r)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(z)
		if err != nil {
			t.Fatal(err)
		}
		if r.Len() != 0 {
			t.Errorf("at %d px: %d bytes after the zlib stream", digit, r.Len())
		}
		if len(raw) != c.Height*(1+c.Width) {
			t.Fatalf("at %d px: %d bytes of rows, want %d", digit, len(raw), c.Height*(1+c.Width))
		}
		for y := range c.Height {
			if f := raw[y*(1+c.Width)]; f > 4 {
				t.Errorf("at %d px: row %d has the filter %d", digit, y, f)
			}
		}
	}
}
