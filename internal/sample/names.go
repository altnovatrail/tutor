package sample

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// sheetName is the name of a saved sheet's file, as the record mode of the board gives it (R25):
// NN-YYYYMMDD-HHMMSS-mmm-task-scenario.json — the number of its plan sheet in two digits, the time
// of the save in UTC to the millisecond, the task and the scenario. The start of the name is of
// fixed width, so the last file of a sheet is the last by name.
var sheetName = regexp.MustCompile(`^(\d{2})-(\d{8}-\d{6})-(\d{3})-([a-z0-9-]+)\.json$`)

// SheetName is what the name of a saved sheet's file says.
type SheetName struct {
	// Number is the number of the plan sheet the file saves.
	Number int
	// At is the time of the save, to the millisecond.
	At time.Time
	// Of is the task and the scenario as the name holds them, joined by a hyphen.
	Of string
}

// SheetFile is the name of the file of a plan sheet saved at the moment at.
func SheetFile(sh *PlanSheet, at time.Time) string {
	at = at.UTC()
	return fmt.Sprintf("%02d-%s-%03d-%s-%s.json", sh.Number, at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), sh.Task, sh.Scenario)
}

// ParseSheetFile reads the name of a saved sheet's file; ok is false for any other name.
func ParseSheetFile(name string) (n SheetName, ok bool) {
	m := sheetName.FindStringSubmatch(name)
	if m == nil {
		return SheetName{}, false
	}
	at, err := time.Parse("20060102-150405", m[2])
	if err != nil {
		return SheetName{}, false
	}
	ms, _ := strconv.Atoi(m[3])
	number, _ := strconv.Atoi(m[1])
	return SheetName{Number: number, At: at.Add(time.Duration(ms) * time.Millisecond), Of: m[4]}, true
}

// Saves reports whether the name is that of a saved file of the plan sheet: its number, its task
// and its scenario. A file of another plan's sheet of the same number does not save it.
func (n SheetName) Saves(sh *PlanSheet) bool {
	return n.Number == sh.Number && n.Of == sh.Task+"-"+string(sh.Scenario)
}

// MaxSheets is the most sheets a plan of the record mode may have: the name of a saved sheet's
// file holds the sheet's number in two digits.
const MaxSheets = 99
