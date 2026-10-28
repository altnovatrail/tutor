package provider

// Place is where a box that a model gave in pixels of a picture stands on it, as an adapter tells
// it (R53 spec): [ymin, xmin, ymax, xmax] in whole thousandths of the picture's height and width,
// from 0 to 1000, the low corner rounded down and the high one up. The box, x1, y1 its top-left
// corner and x2, y2 its bottom-right, in order, is first clamped to the picture of width × height
// px, and divided by that size, the size of the picture sent and not one padded to a multiple of
// 28 px, as R53 spec reads Anthropic's guide to coordinates. A box with no area on the picture, or
// a picture of no size, has no place.
func Place(x1, y1, x2, y2, width, height int) ([4]int64, bool) {
	w, h := int64(width), int64(height)
	if w < 1 || h < 1 {
		return [4]int64{}, false
	}
	clamp := func(v int, hi int64) int64 { return min(max(int64(v), 0), hi) }
	cx1, cx2, cy1, cy2 := clamp(x1, w), clamp(x2, w), clamp(y1, h), clamp(y2, h)
	if cx1 >= cx2 || cy1 >= cy2 {
		return [4]int64{}, false
	}
	return [4]int64{1000 * cy1 / h, 1000 * cx1 / w, (1000*cy2 + h - 1) / h, (1000*cx2 + w - 1) / w}, true
}
