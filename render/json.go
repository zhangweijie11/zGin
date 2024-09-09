package render

import (
	"github.com/zhangweijie11/zGin/internal/json"
	"net/http"
)

type JSON struct {
	Data any
}

var (
	jsonContentType      = []string{"application/json; charset=utf-8"}
	jsonpContentType     = []string{"application/javascript; charset=utf-8"}
	jsonASCIIContentType = []string{"application/json"}
)

func WriteJSON(w http.ResponseWriter, obj any) error {
	writeContentType(w, jsonContentType)
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonBytes)
	return err
}

// WriteContentType (JSON) 写入 json 数据
func (r JSON) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, jsonContentType)
}

// Render (JSON) 使用自定义 ContentType 写入数据
func (r JSON) Render(w http.ResponseWriter) error {
	return WriteJSON(w, r.Data)
}
