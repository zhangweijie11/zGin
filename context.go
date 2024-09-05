package gin

import (
	"errors"
	"math"
	"net"
	"net/http"
	"strings"
)

type Context struct {
	engine      *Engine
	params      *Params
	skippedNode *[]skippedNode
	Request     *http.Request
	Writer      ResponseWriter
	index       int8           // 处理器索引
	handlers    HandlersChain  // 处理器链路
	Keys        map[string]any // 处理请求上下文的键值对
	Errors      errorMsgs      // 错误信息
}

const abortIndex = math.MaxInt8 >> 1

// Next 仅在中间件中使用，将所有处理器都执行一遍
func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		if c.handlers[c.index] != nil {
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
