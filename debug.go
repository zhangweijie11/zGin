package gin

import "sync/atomic"

func IsDebugging() bool {
	return atomic.LoadInt32(&zGinMode) == debugCode
}
