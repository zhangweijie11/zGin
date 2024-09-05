package gin

import "net/http"

const (
	noWritten     = -1
	defaultStatus = http.StatusOK
)

type ResponseWriter interface {
	http.ResponseWriter
	http.Hijacker
	http.Flusher
	Status() int     // 当前请求的响应状态码
	Size() int       // 当前请求的响应体长度
	WriteHeaderNow() // 写入状态码\响应头\响应体
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.size != noWritten
}

func (w *responseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
		w.ResponseWriter.WriteHeader(w.status)
	}
}
