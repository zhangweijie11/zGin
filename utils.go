package gin

import (
	"path"
	"reflect"
	"runtime"
)

func assert1(guard bool, text string) {
	if !guard {
		panic(text)
	}
}

// 将字符串变为 uint8 类型
func lastChar(str string) uint8 {
	if str == "" {
		panic("字符串长度不可以为 0")
	}

	return str[len(str)-1]
}

// 组合请求路径，绝对基本路径+请求路径
func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}

	return finalPath
}

// 获取处理器的名称
func nameOfFunction(f any) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}
