package gin

// 辅助函数来延迟创建缓冲区
func bufApp(buf *[]byte, s string, w int, c byte) {
	b := *buf
	if len(b) == 0 {
		// 到目前为止，没有对原始字符串进行修改。如果下一个字符与原始字符串中的字符相同，则我们还不必分配缓冲区
		if s[w] == c {
			return
		}
		// 请使用堆栈缓冲区（如果它足够大），或者在堆上分配新缓冲区，并复制所有以前的字符
		length := len(s)
		if length > cap(b) {
			*buf = make([]byte, length)
		} else {
			*buf = (*buf)[:length]
		}

		b = *buf

		copy(b, s[:w])
	}
	b[w] = c
}

// 清理并规范化给定的URL路径。它移除多余的斜杠、处理.和..路径段，并确保路径以单个斜杠开头
func cleanPath(p string) string {
	const stackBufSize = 128
	// 如果路由为空，则返回 /
	if p == "" {
		return "/"
	}

	buf := make([]byte, 0, stackBufSize)

	n := len(p)

	r := 1
	w := 1

	if p[0] != '/' {
		r = 0
		if n+1 > stackBufSize {
			buf = make([]byte, n+1)
		} else {
			buf = buf[:n+1]
		}
		buf[0] = '/'
	}

	trailing := n > 1 && p[n-1] == '/'

	for r < n {
		switch {
		case p[r] == '/':
			// 空路由，则在末尾后添加尾部斜杠
			r++
		case p[r] == '.' && r+1 == n:
			trailing = true
			r++
		case p[r] == '.' && p[r+1] == '/':
			// . 路由
			r += 2
		case p[r] == '.' && p[r+1] == '.' && (r+2 == n || p[r+2] == '/'):
			// 路由中存在 ..，移除最后的 /
			r += 3
			if w > 1 {
				w--
				if len(buf) == 0 {
					for w > 1 && p[w] != '/' {
						w--
					}
				} else {
					for w > 1 && buf[w] != '/' {
						w--
					}
				}
			}
		default:
			if w > 1 {
				bufApp(&buf, p, w, '/')
				w++
			}

			for r < n && p[r] != '/' {
				bufApp(&buf, p, w, p[r])
				w++
				r++
			}
		}
	}

	if trailing && w > 1 {
		bufApp(&buf, p, w, '/')
		w++
	}

	if len(buf) == 0 {
		return p[:w]
	}

	return string(buf[:w])
}
