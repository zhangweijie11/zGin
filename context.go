package gin

import (
	"errors"
	"github.com/zhangweijie11/zGin/binding"
	"github.com/zhangweijie11/zGin/render"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Context struct {
	engine       *Engine
	params       *Params
	skippedNodes *[]skippedNode
	Request      *http.Request
	Writer       ResponseWriter
	index        int8           // 处理器索引
	handlers     HandlersChain  // 处理器链路
	Keys         map[string]any // 处理请求上下文的键值对
	Errors       errorMsgs      // 错误信息
	writermem    responseWriter // 自定义响应写入
	Params       Params         //
	fullPath     string         // 完整路由
	Accepted     []string       // 定义用于内容协商的手动接受格式列表
	queryCache   url.Values     // 缓存来自 c.Request.URL.Query（） 的查询结果
	formCache    url.Values     // 缓存 c.Request.PostForm，其中包含来自 POST、PATCH 或 PUT 正文参数的解析表单数据
	sameSite     http.SameSite  // 允许服务器定义 cookie 属性，使其成为浏览器与跨站点请求一起发送此 cookie
	mu           sync.RWMutex   // 获取请求上下文时加锁
}

const abortIndex = math.MaxInt8 >> 1
const (
	MIMEJSON              = binding.MIMEJSON
	MIMEHTML              = binding.MIMEHTML
	MIMEXML               = binding.MIMEXML
	MIMEXML2              = binding.MIMEXML2
	MIMEPlain             = binding.MIMEPlain
	MIMEPOSTForm          = binding.MIMEPOSTForm
	MIMEMultipartPOSTForm = binding.MIMEMultipartPOSTForm
	MIMEYAML              = binding.MIMEYAML
	MIMEYAML2             = binding.MIMEYAML2
	MIMETOML              = binding.MIMETOML
)

// BodyBytesKey 默认请求上下文字节数据
const BodyBytesKey = "_gin-gonic/gin/bodybyteskey"

// ContextKey 默认请求上下文
const ContextKey = "_gin-gonic/gin/contextkey"

// Next 仅在中间件中使用，将所有处理器都执行一遍
func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		if c.handlers[c.index] == nil {
			continue
		}
		c.handlers[c.index](c)
		c.index++
	}
}

// Get 根据键在请求上下文获取值
func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.Keys[key]
	return
}

// Set 在请求上下文设置键值对
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	c.Keys[key] = value
}

func (c *Context) requestHeader(key string) string {
	return c.Request.Header.Get(key)
}

// RemoteIP 获取远程 发起请求的 IP
func (c *Context) RemoteIP() string {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil {
		return ""
	}
	return ip
}

// ClientIP 获取客户端请求 IP
func (c *Context) ClientIP() string {
	if c.engine.TrustedPlatform != "" {
		// 开发人员可以定义自己的 Trusted Platform 标头或使用预定义的常量
		if addr := c.requestHeader(c.engine.TrustedPlatform); addr != "" {
			return addr
		}
	}

	remoteIP := net.ParseIP(c.RemoteIP())
	if remoteIP == nil {
		return ""
	}

	// 判断 IP 是否可信任（一共有两处可以获得 IP，一个是远程请求的 IP，一个是包含在请求头中的 IP（优先级高））
	trusted := c.engine.isTrustedProxy(remoteIP)
	if trusted && c.engine.ForwardedByClientIP && c.engine.RemoteIPHeaders != nil {
		for _, headerName := range c.engine.RemoteIPHeaders {
			ip, valid := c.engine.validateHeader(c.requestHeader(headerName))
			if valid {
				return ip
			}
		}
	}
	return remoteIP.String()
}

// 收集错误，推送到上下文
func (c *Context) Error(err error) *Error {
	if err == nil {
		panic("err is nil")
	}

	var parsedError *Error
	ok := errors.As(err, &parsedError)
	if !ok {
		parsedError = &Error{
			Err:  err,
			Type: ErrorTypePrivate,
		}
	}
	c.Errors = append(c.Errors, parsedError)
	return parsedError
}

// Abort 停止调用接下来的处理器
func (c *Context) Abort() {
	c.index = abortIndex
}

// 重置请求上下文
func (c *Context) reset() {
	c.Writer = &c.writermem
	c.Params = c.Params[:0]
	c.handlers = nil
	c.index = -1
	c.fullPath = ""
	c.Keys = nil
	c.Errors = c.Errors[:0]
	c.Accepted = nil
	c.queryCache = nil
	c.formCache = nil
	c.sameSite = 0
	*c.params = (*c.params)[:0]
	*c.skippedNodes = (*c.skippedNodes)[:0]

}

// http.bodyAllowedForStatus的副本函数
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}

// Status 将响应状态码写入响应头
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

// AbortWithStatus 停止响应
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Writer.WriteHeaderNow()
	c.Abort()
}

// Render 写入响应标头并呈现数据
func (c *Context) Render(code int, r render.Render) {
	c.Status(code)

	// 如果不是涉及到响应体的状态码，就正常返回
	if !bodyAllowedForStatus(code) {
		r.WriteContentType(c.Writer)
		c.Writer.WriteHeaderNow()
		return
	}

	if err := r.Render(c.Writer); err != nil {
		_ = c.Error(err)
		c.Abort()
	}
}

// String 返回字符串响应体
func (c *Context) String(code int, format string, value ...any) {
	c.Render(code, render.String{Format: format, Data: value})
}

// JSON 返回 json 类型响应体
func (c *Context) JSON(code int, value ...any) {
	c.Render(code, render.JSON{Data: value})
}

// ContentType 获取ContentType
func (c *Context) ContentType() string {
	return filterFlags(c.requestHeader("Content-Type"))
}

// ShouldBindWith 绑定请求数据到指定的结构体，可以绑定多种类型数据，允许多次调用，每次调用都会重新读取和解析请求体
func (c *Context) ShouldBindWith(obj any, b binding.Binding) error {
	return b.Bind(c.Request, obj)
}

// ShouldBindBodyWith 绑定请求数据到指定的结构体，但是和 ShouldBindWith 不同，只会读取请求体一次，并且会将请求体的内容缓存起来，以便后续操作可以重复使用数据，如果在处理过程中多次使用请求体数据，使用这个方法更为高效
func (c *Context) ShouldBindBodyWith(obj any, bb binding.BindingBody) (err error) {
	var body []byte
	if cb, ok := c.Get(BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}
	if body == nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return err
		}
		c.Set(BodyBytesKey, body)
	}
	return bb.BindBody(body, obj)
}

// ShouldBind 根据 Method 和 ContentType 判断使用哪种参数绑定方法
func (c *Context) ShouldBind(obj any) error {
	b := binding.Default(c.Request.Method, c.ContentType())
	return c.ShouldBindWith(obj, b)
}

// ShouldBindJSON 绑定 json 请求数据到结构体
func (c *Context) ShouldBindJSON(obj any) error {
	return c.ShouldBindWith(obj, binding.JSON)
}
