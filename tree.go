package gin

import (
	"bytes"
	"github.com/zhangweijie11/zGin/iinternal/bytesconv"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	static nodeType = iota
	root
	param
	catchAll
)

var (
	strColon = []byte(":")
	strStar  = []byte("*")
	strSlash = []byte("/")
)

type Param struct {
	Key   string
	Value string
}

type Params []Param

type nodeType uint8

type node struct {
	path      string        // 路由路径
	indices   string        // children 中各个 path 的开头第一个字符的集合
	wildChild bool          // 是否正则匹配
	nType     nodeType      // 节点类型
	priority  uint32        // 优先级
	children  []*node       // 子节点
	handlers  HandlersChain // 处理器流
	fullPath  string        // 完整路由路径
}

// 节点基础数据
type nodeValue struct {
	handlers HandlersChain
	params   *Params
	tsr      bool
	fullPath string
}

type skippedNode struct {
	path        string
	node        *node
	paramsCount int16
}

type methodTree struct {
	method string
	root   *node
}

type methodTrees []methodTree

// 根据请求方法获取请求路径
func (trees methodTrees) get(method string) *node {
	for _, tree := range trees {
		if tree.method == method {
			return tree.root
		}
	}
	return nil
}

// 查找是否有转义字符串或者通配符
func findWildcard(path string) (wildcard string, i int, valid bool) {
	// 是否存在转义字符标志位
	escapeColon := false
	for start, c := range []byte(path) {
		if escapeColon {
			escapeColon = false
			if c == ':' {
				continue
			}
			panic("路由中存在转义字符串或通配符 '" + path + "'")
		}
		if c == '\\' {
			escapeColon = true
			continue
		}
		// 通配符以 : 或 * 开头
		if c != ':' && c != '*' {
			continue
		}

		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}

// 添加子节点，将 wildcardChild 保留在末尾
func (n *node) addChild(child *node) {
	if n.wildChild && len(n.children) > 0 {
		wildcardChild := n.children[len(n.children)-1]
		n.children = append(n.children[:len(n.children)-1], child, wildcardChild)
	} else {
		n.children = append(n.children, child)
	}
}

func (n *node) insertChild(path string, fullPath string, handlers HandlersChain) {
	for {
		// 查找前缀，直到找到第一个通配符
		wildcard, i, valid := findWildcard(path)
		// 如果没有通配符直接退出
		if i < 0 {
			break
		}

		if !valid {
			panic("在路由中只有一个通配符可以被允许使用: '" + wildcard + fullPath + "'")
		}

		if len(wildcard) < 2 {
			panic("路由长度包含通配符需大于 2 '" + fullPath + "'")
		}

		if wildcard[0] == ':' {
			if i > 0 {
				n.path = path[:i]
				path = path[i:]
			}

			child := &node{
				nType:    param,
				path:     wildcard,
				fullPath: fullPath,
			}

			n.addChild(child)
			n.wildChild = true
			n = child
			n.priority++

			if len(wildcard) < len(path) {
				path = path[len(wildcard):]

				child := &node{
					priority: 1,
					fullPath: fullPath,
				}
				n.addChild(child)
				n = child
				continue
			}
			n.handlers = handlers
			return
		}

		if i+len(wildcard) != len(path) {
			panic("只允许在路由末尾添加通配符" + fullPath)
		}

		if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			pathSeg := ""
			if len(n.children) != 0 {
				pathSeg = strings.SplitN(n.children[0].path, "/", 2)[0]
			}
			panic("catch-all wildcard '" + path +
				"' in new path '" + fullPath +
				"' conflicts with existing path segment '" + pathSeg +
				"' in existing prefix '" + n.path + pathSeg +
				"'")
		}
		i--

		if path[i] != '/' {
			panic("no / before catch-all in path '" + fullPath + "'")
		}
		n.path = path[:i]

		child := &node{
			wildChild: true,
			nType:     catchAll,
			fullPath:  fullPath,
		}
		n.addChild(child)
		n.indices = string('/')
		n = child
		n.priority++

		child = &node{
			path:     path[i:],
			nType:    catchAll,
			handlers: handlers,
			priority: 1,
			fullPath: fullPath,
		}
		n.children = []*node{child}
		return

	}

	n.path = path
	n.handlers = handlers
	n.fullPath = fullPath
}

// 获取最长公共前缀的索引值
func longestCommonPrefix(a, b string) int {
	i := 0
	max_ := 0
	if len(a) < len(b) {
		max_ = len(a)
	} else {
		max_ = len(b)
	}
	for i < max_ && a[i] == b[i] {
		i++
	}
	return i

}

// 递增节点的优先级，并在必要时重新排序
func (n *node) incrementChildPrio(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	newPos := pos
	for ; newPos > 0 && cs[newPos-1].priority > prio; newPos-- {
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
	}
	if newPos != pos {
		n.indices = n.indices[:newPos] + n.indices[pos:pos+1] + n.indices[newPos:pos] + n.indices[pos+1:]
	}

	return newPos

}

// 添加路由节点
func (n *node) addRoute(path string, handlers HandlersChain) {
	fullPath := path
	n.priority++

	// 空节点
	if len(n.path) == 0 && len(n.children) == 0 {
		n.insertChild(path, fullPath, handlers)
		n.nType = root
		return
	}

	parentFullPathIndex := 0
walk:
	for {
		// 查找最长的公共前缀,公共前缀不包含 '：' 或 ''，因为现有键不能包含这些字符
		i := longestCommonPrefix(path, n.path)
		if i < len(n.path) {
			child := node{
				path:      n.path[i:],
				wildChild: n.wildChild,
				nType:     static,
				indices:   n.indices,
				children:  n.children,
				handlers:  n.handlers,
				priority:  n.priority - 1,
				fullPath:  n.fullPath,
			}
			n.children = []*node{&child}
			n.indices = bytesconv.BytesToString([]byte{n.path[i]})
			n.path = path[:i]
			n.handlers = nil
			n.wildChild = false
			n.fullPath = fullPath[:parentFullPathIndex+i]
		}

		// 将新节点设为此节点的子节点
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// '/' after param
			if n.nType == param && c == '/' && len(n.children) == 1 {
				parentFullPathIndex += len(n.path)
				n = n.children[0]
				n.priority++
				continue walk
			}

			// 检查是否存在具有下一个路径字节的子项
			for i, max_ := 0, len(n.indices); i < max_; i++ {
				if c == n.indices[i] {
					parentFullPathIndex += len(n.path)
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			if c != ':' && c != '*' && n.nType != catchAll {
				n.indices += bytesconv.BytesToString([]byte{c})
				child := &node{
					fullPath: fullPath,
				}
				n.addChild(child)
				n.incrementChildPrio(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				// 插入通配符节点，需要检查是否与已有的通配符冲突
				n = n.children[len(n.children)-1]
				n.priority++

				// 检查通配符是否匹配
				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					// 无法将子项添加到 catchAll
					n.nType != catchAll &&
					// 检查长通配符, e.g. :name and :names
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}

				// 通配符冲突
				pathSeg := path
				if n.nType != catchAll {
					pathSeg = strings.SplitN(pathSeg, "/", 2)[0]
				}
				prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path
				panic("'" + pathSeg +
					"' in new path '" + fullPath +
					"' conflicts with existing wildcard '" + n.path +
					"' in existing prefix '" + prefix +
					"'")
			}

			n.insertChild(path, fullPath, handlers)
			return
		}

		// 将句柄添加到当前节点
		if n.handlers != nil {
			panic("路由已经存在" + fullPath)
		}
		n.handlers = handlers
		n.fullPath = fullPath
		return
	}
}

func countParams(path string) uint16 {
	var n uint16
	s := bytesconv.StringToBytes(path)
	n += uint16(bytes.Count(s, strColon))
	n += uint16(bytes.Count(s, strStar))

	return n
}

func countSections(path string) uint16 {
	s := bytesconv.StringToBytes(path)
	return uint16(bytes.Count(s, strColon))
}

// 通过遍历路径树，匹配路径前缀、处理路径参数和通配符路径，查找与路径匹配的处理器。
// 它还处理路径回退逻辑，以应对路径查找失败的情况，并推荐路径重定向。
// 这个函数在路由树中查找路径并返回查找结果，包括路径参数、处理器和路径重定向推荐
func (n *node) getValue(path string, params *Params, skippedNodes *[]skippedNode, unescape bool) (value nodeValue) {
	var globalParamsCount int16

walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				// 遍历当前节点的子节点，如果找到匹配的子节点，更新当前节点并继续循环。
				idxc := path[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
						if n.wildChild {
							index := len(*skippedNodes)
							*skippedNodes = (*skippedNodes)[:index+1]
							(*skippedNodes)[index] = skippedNode{
								path: prefix + path,
								node: &node{
									path:      n.path,
									wildChild: n.wildChild,
									nType:     n.nType,
									priority:  n.priority,
									children:  n.children,
									handlers:  n.handlers,
									fullPath:  n.fullPath,
								},
								paramsCount: globalParamsCount,
							}
						}

						n = n.children[i]
						continue walk
					}
				}

				if !n.wildChild {
					// 如果没有匹配的子节点并且当前节点不是通配符节点，尝试回退到上一个有效的节点。如果找到通配符子节点，更新当前节点。
					if path != "/" {
						for length := len(*skippedNodes); length > 0; length-- {
							skippedNode := (*skippedNodes)[length-1]
							*skippedNodes = (*skippedNodes)[:length-1]
							if strings.HasSuffix(skippedNode.path, path) {
								path = skippedNode.path
								n = skippedNode.node
								if value.params != nil {
									*value.params = (*value.params)[:skippedNode.paramsCount]
								}
								globalParamsCount = skippedNode.paramsCount
								continue walk
							}
						}
					}

					// 没有找到匹配的节点，返回 `tsr` 推荐
					value.tsr = path == "/" && n.handlers != nil
					return value
				}

				n = n.children[len(n.children)-1]
				globalParamsCount++

				switch n.nType {
				// 找到参数的结尾并保存参数值。如果还有路径段，继续深入路径树。否则，检查是否有处理器并返回结果
				case param:
					// 处理路径参数
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}

					// 保存参数值
					if params != nil {
						if cap(*params) < int(globalParamsCount) {
							newParams := make(Params, len(*params), globalParamsCount)
							copy(newParams, *params)
							*params = newParams
						}

						if value.params == nil {
							value.params = params
						}
						i := len(*value.params)
						*value.params = (*value.params)[:i+1]
						val := path[:end]
						if unescape {
							if v, err := url.QueryUnescape(val); err == nil {
								val = v
							}
						}
						(*value.params)[i] = Param{
							Key:   n.path[1:],
							Value: val,
						}
					}

					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}

						value.tsr = len(path) == end+1
						return value
					}

					if value.handlers = n.handlers; value.handlers != nil {
						value.fullPath = n.fullPath
						return value
					}
					if len(n.children) == 1 {
						n = n.children[0]
						value.tsr = (n.path == "/" && n.handlers != nil) || (n.path == "" && n.indices == "/")
					}
					return value

					// 保存捕获所有参数值。返回找到的处理器和完整路径。
				case catchAll:
					if params != nil {
						if cap(*params) < int(globalParamsCount) {
							newParams := make(Params, len(*params), globalParamsCount)
							copy(newParams, *params)
							*params = newParams
						}

						if value.params == nil {
							value.params = params
						}
						i := len(*value.params)
						*value.params = (*value.params)[:i+1]
						val := path
						if unescape {
							if v, err := url.QueryUnescape(path); err == nil {
								val = v
							}
						}
						(*value.params)[i] = Param{
							Key:   n.path[2:],
							Value: val,
						}
					}

					value.handlers = n.handlers
					value.fullPath = n.fullPath
					return value

				default:
					panic("invalid node type")
				}
			}
		}
		// 如果当前路径与节点前缀匹配，检查节点是否有处理器。
		// 如果没有处理器，尝试从跳过的节点中回退。
		// 如果找到处理器，返回结果。
		// 如果路径是 / 并且有通配符子节点或静态节点，设置 tsr 为 true。
		// 检查路径末尾是否需要添加斜杠的推荐
		if path == prefix {
			if n.handlers == nil && path != "/" {
				for length := len(*skippedNodes); length > 0; length-- {
					skippedNode := (*skippedNodes)[length-1]
					*skippedNodes = (*skippedNodes)[:length-1]
					if strings.HasSuffix(skippedNode.path, path) {
						path = skippedNode.path
						n = skippedNode.node
						if value.params != nil {
							*value.params = (*value.params)[:skippedNode.paramsCount]
						}
						globalParamsCount = skippedNode.paramsCount
						continue walk
					}
				}
				//	n = latestNode.children[len(latestNode.children)-1]
			}
			if value.handlers = n.handlers; value.handlers != nil {
				value.fullPath = n.fullPath
				return value
			}

			if path == "/" && n.wildChild && n.nType != root {
				value.tsr = true
				return value
			}

			if path == "/" && n.nType == static {
				value.tsr = true
				return value
			}

			for i, c := range []byte(n.indices) {
				if c == '/' {
					n = n.children[i]
					value.tsr = (len(n.path) == 1 && n.handlers != nil) ||
						(n.nType == catchAll && n.children[0].handlers != nil)
					return value
				}
			}

			return value
		}

		// 如果没有找到匹配的节点，设置 tsr 为 true，推荐添加斜杠的重定向。尝试从跳过的节点中回退。返回最终结果。
		value.tsr = path == "/" ||
			(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
				path == prefix[:len(prefix)-1] && n.handlers != nil)

		if !value.tsr && path != "/" {
			for length := len(*skippedNodes); length > 0; length-- {
				skippedNode := (*skippedNodes)[length-1]
				*skippedNodes = (*skippedNodes)[:length-1]
				if strings.HasSuffix(skippedNode.path, path) {
					path = skippedNode.path
					n = skippedNode.node
					if value.params != nil {
						*value.params = (*value.params)[:skippedNode.paramsCount]
					}
					globalParamsCount = skippedNode.paramsCount
					continue walk
				}
			}
		}

		return value
	}
}

// 不区分大小写查找路由
func (n *node) findCaseInsensitivePath(path string, fixTrailingSlash bool) ([]byte, bool) {
	const stackBufSize = 128
	buf := make([]byte, 0, stackBufSize)
	if length := len(path) + 1; length > stackBufSize {
		buf = make([]byte, 0, length)
	}
	ciPath := n.findCaseInsensitivePathRec(path, buf, [4]byte{}, fixTrailingSlash)

	return ciPath, ciPath != nil
}

// 将数组中的字节向左移动 n 个字节
func shiftNRuneBytes(rb [4]byte, n int) [4]byte {
	switch n {
	case 0:
		return rb
	case 1:
		return [4]byte{rb[1], rb[2], rb[3], 0}
	case 2:
		return [4]byte{rb[2], rb[3]}
	case 3:
		return [4]byte{rb[3]}
	default:
		return [4]byte{}
	}
}

// 通过递归遍历路径树，忽略大小写匹配路径。
// 它处理普通节点、参数节点和通配符节点，并尝试修复尾部斜杠。
// 该函数返回大小写不敏感的路径，并在路径树中查找到匹配的节点。
func (n *node) findCaseInsensitivePathRec(path string, ciPath []byte, rb [4]byte, fixTrailingSlash bool) []byte {
	npLen := len(n.path)

walk:
	for len(path) >= npLen && (npLen == 0 || strings.EqualFold(path[1:npLen], n.path[1:])) {
		// 保存当前路径，并去掉匹配的前缀部分。将节点路径添加到结果路径中。
		oldPath := path
		path = path[npLen:]
		ciPath = append(ciPath, n.path...)

		// 如果路径已经处理完毕，检查当前节点是否有处理器。如果有，返回结果路径。如果没有处理器，尝试通过添加尾部斜杠来修复路径。
		if len(path) == 0 {
			if n.handlers != nil {
				return ciPath
			}

			if fixTrailingSlash {
				for i, c := range []byte(n.indices) {
					if c == '/' {
						n = n.children[i]
						if (len(n.path) == 1 && n.handlers != nil) ||
							(n.nType == catchAll && n.children[0].handlers != nil) {
							return append(ciPath, '/')
						}
						return nil
					}
				}
			}
			return nil
		}

		// 如果当前节点没有通配符子节点，处理下一个字符。
		// 先跳过已经处理的字符，如果缓存中有字符，查找匹配的子节点。
		// 如果没有缓存字符，处理新的字符并查找匹配的子节点。
		// 如果找到小写字符匹配的子节点，递归查找。
		// 如果找到大写字符匹配的子节点，更新当前节点。
		// 如果未找到任何匹配，尝试建议添加尾部斜杠。
		if !n.wildChild {
			rb = shiftNRuneBytes(rb, npLen)

			if rb[0] != 0 {
				idxc := rb[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
						n = n.children[i]
						npLen = len(n.path)
						continue walk
					}
				}
			} else {
				var rv rune
				var off int
				for max_ := min(npLen, 3); off < max_; off++ {
					if i := npLen - off; utf8.RuneStart(oldPath[i]) {
						// read rune from cached path
						rv, _ = utf8.DecodeRuneInString(oldPath[i:])
						break
					}
				}

				lo := unicode.ToLower(rv)
				utf8.EncodeRune(rb[:], lo)

				rb = shiftNRuneBytes(rb, off)

				idxc := rb[0]
				for i, c := range []byte(n.indices) {
					if c == idxc {
						if out := n.children[i].findCaseInsensitivePathRec(
							path, ciPath, rb, fixTrailingSlash,
						); out != nil {
							return out
						}
						break
					}
				}

				if up := unicode.ToUpper(rv); up != lo {
					utf8.EncodeRune(rb[:], up)
					rb = shiftNRuneBytes(rb, off)

					idxc := rb[0]
					for i, c := range []byte(n.indices) {
						if c == idxc {
							n = n.children[i]
							npLen = len(n.path)
							continue walk
						}
					}
				}
			}

			if fixTrailingSlash && path == "/" && n.handlers != nil {
				return ciPath
			}
			return nil
		}

		// 如果当前节点有通配符子节点，处理通配符节点。
		// 对于 param 节点，找到参数的结尾并添加到结果路径中。
		// 如果路径未处理完毕，继续处理子节点。
		// 如果路径处理完毕，检查是否有处理器或尝试建议添加尾部斜杠。
		// 对于 catchAll 节点，添加剩余路径到结果路径中。
		n = n.children[0]
		switch n.nType {
		case param:
			end := 0
			for end < len(path) && path[end] != '/' {
				end++
			}

			ciPath = append(ciPath, path[:end]...)

			if end < len(path) {
				if len(n.children) > 0 {
					n = n.children[0]
					npLen = len(n.path)
					path = path[end:]
					continue
				}

				if fixTrailingSlash && len(path) == end+1 {
					return ciPath
				}
				return nil
			}

			if n.handlers != nil {
				return ciPath
			}

			if fixTrailingSlash && len(n.children) == 1 {
				n = n.children[0]
				if n.path == "/" && n.handlers != nil {
					return append(ciPath, '/')
				}
			}

			return nil

		case catchAll:
			return append(ciPath, path...)

		default:
			panic("invalid node type")
		}
	}

	// 如果未找到匹配的节点，尝试通过添加或移除尾部斜杠来修复路径。
	if fixTrailingSlash {
		if path == "/" {
			return ciPath
		}
		if len(path)+1 == npLen && n.path[len(path)] == '/' &&
			strings.EqualFold(path[1:], n.path[1:len(path)]) && n.handlers != nil {
			return append(ciPath, n.path...)
		}
	}
	return nil
}
