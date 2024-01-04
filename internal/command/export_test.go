package command

import "go.octolab.org/toolset/pulse/internal/watch"

// SetLoader replaces what loads the lists for a test and returns the way
// back.
func SetLoader(loader watch.Loader) (restore func()) {
	previous := newLoader
	newLoader = func() watch.Loader { return loader }
	return func() { newLoader = previous }
}
