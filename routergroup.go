package gin

// RouterGroup 路由组，路由前缀
type RouterGroup struct {
	Handlers HandlersChain
	basePath string
	engine   *Engine
	root     bool
}

type IRoutes interface {
	Use(...HandlerFunc) IRoutes
}

func (group *RouterGroup) Use(middleware ...HandlerFunc) IRoutes {
	group.Handlers = append(group.Handlers, middleware...)
	return group.returnObj()
}

func (group *RouterGroup) returnObj() IRoutes {
	if group.root {
		return group.engine
	}

	return group
}

// 组装处理器
func (group *RouterGroup) combineHandlers(handlers HandlersChain) HandlersChain {
	finalSize := len(group.Handlers) + len(handlers)
	assert1(finalSize < int(abortIndex), "处理器数量太多")
	mergeHandlers := make(HandlersChain, finalSize)
	copy(mergeHandlers, group.Handlers)
	copy(mergeHandlers[len(group.Handlers):], handlers)
	return mergeHandlers
}
