package ginS

import (
	gin "github.com/zhangweijie11/zGin"
	"sync"
)

var once sync.Once
var internalEngine *gin.Engine

// 单例模式创建基础引擎
func engine() *gin.Engine {
	once.Do(func() {
		internalEngine = gin.Default()
	})

	return internalEngine
}

func Run(addr ...string) (err error) {
	return engine().Run(addr...)
}
