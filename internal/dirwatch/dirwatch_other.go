//go:build !linux

package dirwatch

import "context"

// Supported reports whether this build can watch a directory through events.
const Supported = false

func watch(context.Context, string, bool, func()) bool { return false }
