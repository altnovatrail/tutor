package runner

import (
	"regexp"
	"runtime/debug"
)

// Refused is a sample the runner refuses, as the record of a run names it, for the runner's report
// (contracts/README.md of spec, sections 13 and 14; R74): its set, for a run of several, its ID,
// when its file names one that can be read, its file, the reason as the protocol names it, such as
// REFUSAL_CHILD, and what of the sample it is, in words.
type Refused struct {
	Set    string `json:"set,omitempty"`
	Sample string `json:"sample,omitempty"`
	File   string `json:"file"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Versions are the tag of tutor's release this build is and the version of module-kit it holds,
// as the runner's report names them (R74), from the build's information. A build that is no
// release of tutor — from a working tree, or by go run — is v0.0.0, as spec's examples of a report
// are, which no runner made; so is a module the build does not name.
func Versions() (tutor, moduleKit string) {
	info, _ := debug.ReadBuildInfo()
	return versionsOf(info)
}

// versionsOf are the versions a build's information names, NoRelease for none.
func versionsOf(info *debug.BuildInfo) (tutor, moduleKit string) {
	tutor, moduleKit = NoRelease, NoRelease
	if info == nil {
		return tutor, moduleKit
	}
	if tag.MatchString(info.Main.Version) {
		tutor = info.Main.Version
	}
	for _, d := range info.Deps {
		if d.Path != ModuleKit {
			continue
		}
		// A module replaced is the replacement's: a folder of code is no release, nor a fork's tag.
		if d.Replace == nil && tag.MatchString(d.Version) {
			moduleKit = d.Version
		}
	}
	return tutor, moduleKit
}

// StoppedByCap begins the reason a run stopped when its cap stopped it, -max-usd: what it did not
// reach then is the limit of the runner's report (R74).
const StoppedByCap = "the cap"

// NoRelease is the version of a build that is no release.
const NoRelease = "v0.0.0"

// ModuleKit is the path of module-kit's Go module, whose verdicts the runner gives.
const ModuleKit = "github.com/altnovatrail/module-kit"

// tag is a tag of a release: vMAJOR.MINOR.PATCH, and nothing more.
var tag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
