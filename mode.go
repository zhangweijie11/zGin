package gin

import (
	"flag"
	"github.com/zhangweijie11/zGin/binding"
	"io"
	"os"
	"sync/atomic"
)

// DefaultWriter 默认写入器
var DefaultWriter io.Writer = os.Stdout

// DefaultErrorWriter 默认错误写入器
var DefaultErrorWriter io.Writer = os.Stderr

const (
	EnvZGinMode = "ZGin_MODE"
	DebugMode   = "debug"
	ReleaseMode = "release"
	TestMode    = "test"
	debugCode   = iota
	releaseCode
	testCode
)

var zGinMode int32 = debugCode
var modeName atomic.Value

func init() {
	mode := os.Getenv(EnvZGinMode)
	SetMode(mode)
}

func SetMode(value string) {
	if value == "" {
		if flag.Lookup("test.v") != nil {
			value = TestMode
		} else {
			value = DebugMode
		}
	}

	switch value {
	case DebugMode, "":
		atomic.StoreInt32(&zGinMode, debugCode)
	case ReleaseMode:
		atomic.StoreInt32(&zGinMode, releaseCode)
	case TestMode:
		atomic.StoreInt32(&zGinMode, testCode)
	default:
		panic("无效的 zGin 启动模式，启动模式必须在 debug、release、test 中")
	}
	modeName.Store(value)
}

func DisableBindValidation() {
	binding.Validator = nil
}

func EnableJsonDecoderUseNumber() {
	binding.EnableDecoderUseNumber = true
}

func EnableJsonDecoderDisallowUnknownFields() {
	binding.EnableDecoderDisallowUnknownFields = true
}

func Mode() string {
	return modeName.Load().(string)
}
