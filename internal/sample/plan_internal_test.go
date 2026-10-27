package sample

import "testing"

// A message of the stand-in of a sheet of parts speaks of parts, but keeps the words of the values
// it quotes and of field names such as part_of_line.
func TestPartsNotLines(t *testing.T) {
	for message, want := range map[string]string{
		"2 is not a line of the sheet, 1…4":                      "2 is not a part of the sheet, 1…4",
		"one line is the answer, and the lines follow":           "one part is the answer, and the parts follow",
		`"line 2" is not a stroke of the sample, on line 2`:      `"line 2" is not a stroke of the sample, on part 2`,
		"part_of_line is false, and a line of lines is outlined": "part_of_line is false, and a part of parts is outlined",
	} {
		if got := partsNotLines(message); got != want {
			t.Errorf("partsNotLines(%q) = %q, want %q", message, got, want)
		}
	}
}
