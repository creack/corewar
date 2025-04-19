package assets

import (
	_ "embed"
)

// Various sources for champions found in the wild.
// Pretty-printed and partially currated.
// Sorry, didn't keep track of the origins.
//
//go:embed clean-srcs.tar.gz
var CleanSrcsTargz []byte
