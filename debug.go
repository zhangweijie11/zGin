package gin

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// DebugPrintRouteFunc 打印路由的处理器
var DebugPrintRouteFunc func(httpMethod, absolutePath, handlerName string, nuHandlers int)

// DebugPrintFunc 打印方法
var DebugPrintFunc func(format string, values ...interface{})

func IsDebugging() bool {
	return atomic.LoadInt32(&zGinMode) == debugCode
}

// debug模式下控制台打印
func debugPrint(format string, values ...any) {
	if !IsDebugging() {
		return
	}
	if DebugPrintFunc != nil {
		DebugPrintFunc(format, values...)
		return
	}

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	fmt.Fprintf(DefaultWriter, "[zGIN-debug] "+format, values...)
}

// debug 模式下日志输出
func debugPrintRoute(httpMethod, absolutePath string, handlers HandlersChain) {
	if IsDebugging() {
		nuHandlers := len(handlers)
		handlerName := nameOfFunction(handlers.Last())
		if DebugPrintRouteFunc == nil {
			// 请求方法最大 6 个字符串长度，路由最多 25 个字符串长度
			debugPrint("%-6s %-25s --> %s (%d handlers)\n", httpMethod, absolutePath, handlerName, nuHandlers)
		} else {
			DebugPrintRouteFunc(httpMethod, absolutePath, handlerName, nuHandlers)
		}
	}
}

// debug 模式下错误日志输出
func debugPrintError(err error) {
	if err != nil && IsDebugging() {
		fmt.Fprintf(DefaultErrorWriter, "[zGIN-debug] [ERROR] %v\n", err)
	}
}
