package gin

import (
	"bufio"
	"net"
	"net/http"
)

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

// Hijack 方法用于劫持（hijack）HTTP连接，使得应用程序可以完全控制底层的网络连接。这对于实现 WebSocket 或其他需要低级别网络控制的协议非常有用。
//
// 使用场景
// WebSocket: 在实现 WebSocket 服务器时，通常需要劫持HTTP连接以进行升级握手。
// 自定义协议: 当你需要在 HTTP 之上实现自定义协议时，可以使用 Hijack 方法。
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.size < 0 {
		w.size = 0
	}

	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// Flush 方法用于将 http.ResponseWriter 缓冲区中的数据立即发送到客户端。它通常用于服务器推送数据到客户端的场景，如服务器发送事件（Server-Sent Events, SSE）或分块传输编码（Chunked Transfer Encoding）。
// 使用场景
// 服务器推送: 当服务器需要主动推送数据到客户端，而不是等待客户端请求时，可以使用 Flush 方法。
// 实时更新: 在长连接或实时数据传输的场景中，Flush 可以确保数据及时发送到客户端。
func (w *responseWriter) Flush() {
	w.WriteHeaderNow()
	w.ResponseWriter.(http.Flusher).Flush()
}

// 重置响应上下文
func (w *responseWriter) reset(writer http.ResponseWriter) {
	w.ResponseWriter = writer
	w.size = noWritten
	w.status = defaultStatus
}
