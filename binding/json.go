package binding

import (
	"bytes"
	"errors"
	"github.com/zhangweijie11/zGin/internal/json"
	"io"
	"net/http"
)

// EnableDecoderUseNumber 默认情况下，Go 的 JSON 解码器会将 JSON 数字解码为 float64 类型。但是，float64 类型有精度限制，对于一些非常大的整数或非常精确的小数，可能会出现精度丢失的问题。
// 当调用 decoder.UseNumber() 方法后，解码器将改变其行为，把所有的数字解码为 json.Number 类型。json.Number 实际上是一个字符串类型，用于存储原始的 JSON 数字字符串。这样，可以在后续处理时选择适合的数值类型进行转换，避免浮点数精度丢失的问题。
var EnableDecoderUseNumber = false

// EnableDecoderDisallowUnknownFields 默认情况下，Go 的 JSON 解码器在解码过程中会忽略任何未知字段，不会返回错误。然而，有时候你可能希望确保所有传入的数据都被正确处理，并且没有任何未预期的字段。这时，你可以使用 DisallowUnknownFields 方法来实现这一点。
var EnableDecoderDisallowUnknownFields = false

type jsonBinding struct{}

func (jsonBinding) Name() string {
	return "json"
}

// Bind 绑定参数
func (jsonBinding) Bind(req *http.Request, obj any) error {
	if req == nil || req.Body == nil {
		return errors.New("无效请求")
	}
	return decodeJSON(req.Body, obj)
}

func (jsonBinding) BindBody(body []byte, obj any) error {
	return decodeJSON(bytes.NewReader(body), obj)
}

// json 解码
func decodeJSON(r io.Reader, obj any) error {
	decoder := json.NewDecoder(r)
	if EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(obj); err != nil {
		return err
	}

	return validate(obj)
}
