package gin

import (
	"github.com/zhangweijie11/zGin/iinternal/bytesconv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"net"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
)

const defaultMultipartMemory = 32 << 20 // 32 MB
const escapedColon = "\\:"
const colon = ":"
const backslash = "\\"

var regSafePrefix = regexp.MustCompile("[^a-zA-Z0-9/-]+")
var regRemoveRepeatedChar = regexp.MustCompile("/{2,}")
var mimePlain = []string{MIMEPlain}
var (
	default404Body = []byte("404 page not found")
	default405Body = []byte("405 method not allowed")
)

type HandlerFunc func(*Context)

type HandlersChain []HandlerFunc

type Engine struct {
	RouterGroup
	pool                  sync.Pool
	maxSections           uint16
	maxParams             uint16        // 最大参数长度
	allNoRoute            HandlersChain // 全部未知路由
	allNoMethod           HandlersChain // 全部未知请求类型
	noRoute               HandlersChain // 未知路由
	noMethod              HandlersChain // 未知请求类型
	TrustedPlatform       string        // 是否信任该平台设置的标头,如果设置了则信任
	trustedCIDRs          []*net.IPNet  // 信任的 IP 列表
	ForwardedByClientIP   bool          // 是否允许转发 IP
	RemoteIPHeaders       []string      // 客户端的请求头
	trees                 methodTrees   // 路由树，以请求方法作为key ，该请求方法下的路由树作为 value
	UseH2C                bool          // 是否启用 h2c 支持
	UseRawPath            bool          // 是否可以从URL.RawPath 中查找参数
	UnescapePathValues    bool          // 是否转义 path
	RemoveExtraSlash      bool          // 是否开启即使有额外的斜杠，也可以从 URL 解析参数
	RedirectTrailingSlash bool          // 是否允许重定向
	RedirectFixedPath     bool          // 尝试修复路径进行重定向
	HandleMethodNotAllow  bool          // 是否允许当前请求使用其他方法
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
	return &Context{engine: engine, params: &v, skippedNodes: &skippedNodes}
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

// 判断是否为不安全的代理，例如 0.0.0.0 或者 ::
func (engine *Engine) isUnsafeTrustedProxies() bool {
	return engine.isTrustedProxy(net.ParseIP("0.0.0.0")) || engine.isTrustedProxy(net.ParseIP("::"))
}

// 递归加载最新路由树
func updateRouteTree(n *node) {
	n.path = strings.ReplaceAll(n.path, escapedColon, colon)
	n.fullPath = strings.ReplaceAll(n.fullPath, escapedColon, colon)
	n.indices = strings.ReplaceAll(n.indices, backslash, colon)
	if n.children == nil {
		return
	}
	for _, child := range n.children {
		updateRouteTree(child)
	}
}

// 加载最新路由树
func (engine *Engine) updateRouteTrees() {
	for _, tree := range engine.trees {
		updateRouteTree(tree.root)
	}
}

func redirectFixedPath(c *Context, root *node, trailingSlash bool) bool {
	req := c.Request
	rPath := req.URL.Path

	if fixedPath, ok := root.findCaseInsensitivePath(cleanPath(rPath), trailingSlash); ok {
		req.URL.Path = bytesconv.BytesToString(fixedPath)
		redirectRequest(c)
		return true
	}
	return false
}

func redirectRequest(c *Context) {
	req := c.Request
	rPath := req.URL.Path
	rURL := req.URL.String()

	code := http.StatusMovedPermanently
	if req.Method != http.MethodGet {
		code = http.StatusTemporaryRedirect
	}
	debugPrint("请求重定向 %d: %s --> %s", code, rPath, rURL)
	http.Redirect(c.Writer, req, rURL, code)
	c.writermem.WriteHeaderNow()
}

func redirectTrailingSlash(c *Context) {
	req := c.Request
	p := req.URL.Path
	if prefix := path.Clean(c.Request.Header.Get("X-Forwarded-Prefix")); prefix != "" {
		prefix = regSafePrefix.ReplaceAllString(prefix, "")
		prefix = regRemoveRepeatedChar.ReplaceAllString(prefix, "/")

		p = prefix + "/" + req.URL.Path
	}

	req.URL.Path = p + "/"
	if length := len(p); length > 1 && p[length-1] == '/' {
		req.URL.Path = p[:length-1]
	}
	redirectRequest(c)
}

// 处理请求
func (engine *Engine) handleHTTPRequest(c *Context) {
	httpMethod := c.Request.Method
	rPath := c.Request.URL.Path
	// 不转义路由
	unescape := false
	// 使用路由参数
	if engine.UseRawPath && len(c.Request.URL.RawQuery) > 0 {
		rPath = c.Request.URL.RawQuery
		unescape = engine.UnescapePathValues
	}
	// 处理无效的 URL 路径，规范一下 URL 路径
	if engine.RemoveExtraSlash {
		rPath = cleanPath(rPath)
	}

	t := engine.trees
	for i, tl := 0, len(t); i < tl; i++ {
		// 判断请求方法是否与请求树的方法相同
		if t[i].method != httpMethod {
			continue
		}
		root := t[i].root
		value := root.getValue(rPath, c.params, c.skippedNodes, unescape)
		if value.params != nil {
			c.params = value.params
		}

		if value.handlers != nil {
			c.handlers = value.handlers
			c.fullPath = value.fullPath
			c.Next()
			c.writermem.WriteHeaderNow()
			return
		}
		if httpMethod != http.MethodConnect && rPath != "/" {
			if value.tsr && engine.RedirectTrailingSlash {
				redirectTrailingSlash(c)
				return
			}
			if engine.RedirectFixedPath && redirectFixedPath(c, root, engine.RedirectFixedPath) {
				return
			}
		}
		break
	}
	if engine.HandleMethodNotAllow && len(t) > 0 {
		allowed := make([]string, 0, len(t)-1)
		for _, tree := range engine.trees {
			if tree.method != httpMethod {
				continue
			}
			if value := tree.root.getValue(rPath, nil, c.skippedNodes, unescape); value.params != nil {
				allowed = append(allowed, tree.method)
			}
		}
		if len(allowed) > 0 {
			c.handlers = engine.Handlers
			c.writermem.Header().Set("Allow", strings.Join(allowed, ", "))
			serverError(c, http.StatusMethodNotAllowed, default405Body)
		}
	}
	c.handlers = engine.allNoRoute
	serverError(c, http.StatusNotFound, default404Body)
}

func serverError(c *Context, code int, defaultMessage []byte) {
	c.writermem.status = code
	c.Next()
	if c.writermem.Written() {
		return
	}
	if c.writermem.Status() == code {
		c.writermem.Header()["Content-Type"] = mimePlain
		_, err := c.Writer.Write(defaultMessage)
		if err != nil {
			debugPrint("cannot write message to writer during serve error: %v", err)
			return
		}
	}
	c.writermem.WriteHeaderNow()
}

// 实现标准 Handler 定义的 ServeHTTP 方法
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 获取请求上下文
	c := engine.pool.Get().(*Context)
	// 重置响应上下文
	c.writermem.reset(w)
	c.Request = req
	// 重置请求上下文
	c.reset()

	engine.handleHTTPRequest(c)

	// 将响应数据放到请求上下文中推送出去
	engine.pool.Put(c)
}

// Handler 动态选择合适的 HTTP 处理器，以支持不同的 HTTP 协议
func (engine *Engine) Handler() http.Handler {
	if engine.UseH2C {
		return engine
	}
	// http2.Server 是 Go 标准库中用于处理 HTTP/2 请求的服务器
	h2s := &http2.Server{}
	return h2c.NewHandler(engine, h2s)
}

// Run 启动服务
func (engine *Engine) Run(addr ...string) (err error) {
	defer func() {
		// debug 模式下错误日志输出
		debugPrintError(err)
	}()

	if engine.isUnsafeTrustedProxies() {
		debugPrint("[WARNING] 你信任所有的代理，这是不安全的，我们推荐设置一个新的值！\n" +
			"Please check https://github.com/gin-gonic/gin/blob/master/docs/doc.md#dont-trust-all-proxies for details.")
	}

	// 加载最新的路由树
	engine.updateRouteTrees()
	// 解析服务地址
	address := resolveAddress(addr)
	debugPrint("启动并监听服务 %s\n", address)
	err = http.ListenAndServe(address, engine.Handler())
	return
}
