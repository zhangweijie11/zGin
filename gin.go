package gin

import (
	"net"
	"strings"
	"sync"
)

type HandlerFunc func(*Context)

type HandlersChain []HandlerFunc

type Engine struct {
	RouterGroup
	pool                sync.Pool
	maxSections         uint16
	maxParams           uint16        // 最大参数长度
	allNoRoute          HandlersChain // 全部未知路由
	allNoMethod         HandlersChain // 全部未知请求类型
	noRoute             HandlersChain // 未知路由
	noMethod            HandlersChain // 未知请求类型
	TrustedPlatform     string        // 是否信任该平台设置的标头,如果设置了则信任
	trustedCIDRs        []*net.IPNet  // 信任的 IP 列表
	ForwardedByClientIP bool          // 是否允许转发 IP
	RemoteIPHeaders     []string      // 客户端的请求头
	trees               methodTrees
}

type OptionFunc func(*Engine)

func New(opts ...OptionFunc) *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{Handlers: nil, basePath: "/", root: true},
	}
	engine.RouterGroup.engine = engine
	engine.pool.New = func() any { return engine.allocateContext(engine.maxParams) }
	return engine.With(opts...)
}

// 分配上下文
func (engine *Engine) allocateContext(maxParams uint16) *Context {
	v := make(Params, 0, maxParams)
	skippedNodes := make([]skippedNode, 0, engine.maxSections)
	return &Context{engine: engine, params: &v, skippedNode: &skippedNodes}
}

func (engine *Engine) Use(middleware ...HandlerFunc) IRoutes {
	engine.RouterGroup.Use(middleware...)
	engine.rebuild404Handlers()
	engine.rebuild405Handlers()
	return engine
}

func (engine *Engine) rebuild404Handlers() {
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
}

func (engine *Engine) rebuild405Handlers() {
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

// 判断 IP 是否可信任
func (engine *Engine) isTrustedProxy(ip net.IP) bool {
	if engine.trustedCIDRs == nil {
		return false
	}
	// 如果 IP 在信任的 IP 列表中则放行
	for _, cidr := range engine.trustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// 校验请求头的有效性，主要是为了获取其中的客户端 IP
func (engine *Engine) validateHeader(header string) (clientIP string, valid bool) {
	if header == "" {
		return "", false
	}
	items := strings.Split(header, ",")
	for i := len(items) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(items[i])
		ip := net.ParseIP(ipStr)
		if ip == nil {
			break
		}
		if (i == 0) || (!engine.isTrustedProxy(ip)) {
			return ipStr, true
		}
	}
	return "", false
}

func (engine *Engine) With(opts ...OptionFunc) *Engine {
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

func Default(opts ...OptionFunc) *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine.With(opts...)
}

// Last 获取最后一个处理器（框架处理器的执行顺序为先进先出）
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

func (engine *Engine) addRoute(method, path string, handlers HandlersChain) {
	// 路由开头必须是 /
	assert1(path[0] == '/', "路由必须以 / 开头")
	// 请求方法不能为空
	assert1(method != "", "请求方法不能为空")
	// 至少需要有一个处理器
	assert1(len(handlers) > 0, "路由至少需要一个处理器")
	// debug 模式下输出日志
	debugPrintRoute(method, path, handlers)

	// 获取原始全部路径
	root := engine.trees.get(method)
	if root == nil {
		root = new(node)
		root.fullPath = "/"
		engine.trees = append(engine.trees, methodTree{method: method, root: root})
	}
	root.addRoute(path, handlers)

	if paramsCount := countParams(path); paramsCount > engine.maxParams {
		engine.maxParams = paramsCount
	}
	if sectionsCount := countSections(path); sectionsCount > engine.maxSections {
		engine.maxSections = sectionsCount
	}

}
