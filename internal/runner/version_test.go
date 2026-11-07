package runner

import (
	"runtime/debug"
	"testing"
)

// A build from a tag of tutor names it, and module-kit by its version; a build from a working tree,
// a pseudo-version or no information is no release, v0.0.0.
func TestVersions(t *testing.T) {
	for _, c := range []struct {
		info             *debug.BuildInfo
		tutor, moduleKit string
	}{
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.1.8"}, Deps: []*debug.Module{{Path: ModuleKit, Version: "v0.2.3"}, {Path: "x", Version: "v9.9.9"}}}, "v0.1.8", "v0.2.3"},
		{&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, NoRelease, NoRelease},
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.1.9-0.20261107190000-57d2f72aa5a4"}, Deps: []*debug.Module{{Path: ModuleKit, Version: "v0.2.4-rc.1"}}}, NoRelease, NoRelease},
		{nil, NoRelease, NoRelease},
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.1.8"}, Deps: []*debug.Module{{Path: ModuleKit, Version: "v0.2.3", Replace: &debug.Module{Path: "../module-kit"}}}}, "v0.1.8", NoRelease},
	} {
		if tu, mk := versionsOf(c.info); tu != c.tutor || mk != c.moduleKit {
			t.Errorf("%+v: %s and %s, want %s and %s", c.info, tu, mk, c.tutor, c.moduleKit)
		}
	}
	if tu, mk := Versions(); !tag.MatchString(tu) || !tag.MatchString(mk) {
		t.Errorf("this build is %s with module-kit %s", tu, mk)
	}
}
