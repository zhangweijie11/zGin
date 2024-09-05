package gin

import (
	"io"
	"os"
)

// DefaultWriter 默认写入器
var DefaultWriter io.Writer = os.Stdout

// DefaultErrorWriter 默认错误写入器
var DefaultErrorWriter io.Writer = os.Stderr

const (
	DebugMode   = "debug"
	ReleaseMode = "release"
	TestMode    = "test"
	debugCode   = iota
	releaseCode
	testCode
)

var zGinMode int32 = debugCode
