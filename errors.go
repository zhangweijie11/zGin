package gin

import (
	"fmt"
	"strings"
)

const (
	ErrorTypeBind    ErrorType = 1 << 63
	ErrorTypeRender  ErrorType = 1 << 62
	ErrorTypePrivate ErrorType = 1 << 0
	ErrorTypePublic  ErrorType = 1 << 1
	ErrorTypeAny     ErrorType = 1<<64 - 1
	ErrorTypeNu                = 2
)

type ErrorType uint64

type Error struct {
	Err  error     // 真实错误
	Type ErrorType // 错误类型
	Meta any       // 错误原始数据
}

type errorMsgs []*Error

// IsType 判断错误类型
func (msg *Error) IsType(flags ErrorType) bool {
	return (msg.Type & flags) > 0
}

// ByType 根据错误类型返回错误信息
func (a errorMsgs) ByType(typ ErrorType) errorMsgs {
	if len(a) == 0 {
		return nil
	}
	if typ == ErrorTypeAny {
		return a
	}
	var result errorMsgs
	for _, msg := range a {
		if msg.IsType(typ) {
			result = append(result, msg)
		}
	}
	return result
}

// 组装错误信息
func (a errorMsgs) String() string {
	if len(a) == 0 {
		return ""
	}
	var buffer strings.Builder
	// 组装错误信息
	for i, msg := range a {
		fmt.Fprintf(&buffer, "Error #%02d: %s\n", i+1, msg.Err)
		if msg.Meta != nil {
			fmt.Fprintf(&buffer, "     Meta: %v\n", msg.Meta)
		}
	}

	return buffer.String()
}
