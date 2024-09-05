package gin

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"io"
	"net/http"
	"os"
	"time"
)

type consoleColorModeValue int

const (
	autoColor consoleColorModeValue = iota
	disableColor
	forceColor
)

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

var consoleColorMode = autoColor

type LogFormatterParams struct {
	Request      *http.Request  // 请求数据
	TimeStamp    time.Time      // 请求时间
	StatusCode   int            // 响应状态码
	Latency      time.Duration  // 响应时间
	ClientIP     string         // 原始请求 IP
	Method       string         // 请求方法
	Path         string         // 请求路径
	ErrorMessage string         // 错误消息
	isTerm       bool           // 是否在终端打印
	BodySize     int            // 请求体长度
	Keys         map[string]any // 请求上下文
}

type LogFormatter func(params LogFormatterParams) string

type LoggerConfig struct {
	Formatter LogFormatter // 日志输出格式
	Output    io.Writer    // 日志输出路径
	SkipPaths []string     // 无需输出日志的请求 URL 路径
	Skip      Skipper      // 无需写入日志的
}

type Skipper func(c *Context) bool

func Logger() HandlerFunc {
	return LoggerWithConfig(LoggerConfig{})
}

// StatusCodeColor 根据响应状态码判断日志输出颜色
func (p *LogFormatterParams) StatusCodeColor() string {
	code := p.StatusCode
	switch {
	// 响应状态码大于等于 100 小于 200
	case code >= http.StatusContinue && code < http.StatusOK:
		return white
		// 响应状态码大于等于 200 小于 300
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return green
		// 响应状态码大于等于 300 小于 400
	case code >= http.StatusMultipleChoices && code < http.StatusBadRequest:
		return white
		// 响应状态码大于等于 400 小于 500
	case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}

// MethodColor 根据请求方法判断日志输出颜色
func (p *LogFormatterParams) MethodColor() string {
	method := p.Method

	switch method {
	case http.MethodGet:
		return blue
	case http.MethodPost:
		return cyan
	case http.MethodPut:
		return yellow
	case http.MethodDelete:
		return red
	case http.MethodPatch:
		return green
	case http.MethodHead:
		return magenta
	case http.MethodOptions:
		return white
	default:
		return reset
	}
}

// ResetColor 根据是否重置判断日志输出颜色
func (p *LogFormatterParams) ResetColor() string {
	return reset
}
func (p *LogFormatterParams) IsOutputColor() bool {
	return consoleColorMode == forceColor || (consoleColorMode == autoColor && p.isTerm)
}

// 获取默认的日志输出格式
var defaultLogFormatter = func(param LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	// 当请求响应时间超过一分钟时，修改响应时间的时间类型
	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}

	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)

}

// LoggerWithConfig 实例化日志处理器
func LoggerWithConfig(conf LoggerConfig) HandlerFunc {
	// 设置日志输出格式
	formatter := conf.Formatter
	if formatter == nil {
		formatter = defaultLogFormatter
	}

	// 设置输出路径
	out := conf.Output
	if out == nil {
		out = DefaultWriter
	}

	// 无需输出的请求 URL 路径
	notLlogged := conf.SkipPaths
	// 日志永远输出到控制台
	isTerm := true

	// 检查是否为终端设备，如果不是则无需输出到控制台
	if w, ok := out.(*os.File); !ok || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd())) {
		isTerm = false
	}

	var skip map[string]struct{}

	// 如果无需输出的请求 URL路径数量大于 0，则将其数据放到 skip 中
	if length := len(notLlogged); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range notLlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *Context) {
		// 请求开始时间
		start := time.Now()
		// 请求 URL 路径
		path := c.Request.URL.Path
		// 请求参数
		raw := c.Request.URL.RawQuery

		c.Next()

		if _, ok := skip[path]; ok || (conf.Skip != nil && conf.Skip(c)) {
			return
		}

		param := LogFormatterParams{
			Request: c.Request,
			isTerm:  isTerm,
			Keys:    c.Keys,
		}

		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)
		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(ErrorTypePrivate).String()
		param.BodySize = c.Writer.Size()
		// 组装请求参数到请求路径中
		if raw != "" {
			path = path + "?" + raw
		}
		param.Path = path

		// 将数据写入 out 中
		fmt.Fprint(out, formatter(param))
	}
}

// LoggerWithFormatter 实例化有自己日志输出格式的日志处理器
func LoggerWithFormatter(f LogFormatter) HandlerFunc {
	return LoggerWithConfig(LoggerConfig{
		Formatter: f,
	})
}

// LoggerWithWriter 具有指定写入器缓冲区的日志处理器
func LoggerWithWriter(out io.Writer, notlogged ...string) HandlerFunc {
	return LoggerWithConfig(LoggerConfig{
		Output:    out,
		SkipPaths: notlogged,
	})
}
