package gin

import (
	"os"
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

// 解析服务启动地址
func resolveAddress(addr []string) string {
	switch len(addr) {
	case 0:
		if port := os.Getenv("PORT"); port != "" {
			debugPrint("环境变量有效端口为\"%s\"", port)
			return ":" + port
		}
		debugPrint("环境变量未定义有效端口，使用默认端口 8080 ")
		return ":8080"
	case 1:
		return addr[0]
	default:
		panic("服务地址参数过多")
	}
}

// 根据flag 从 content 中提取数据
func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}

	return content
}
