package gin

import (
	"errors"
	"github.com/zhangweijie11/zGin/binding"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
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
