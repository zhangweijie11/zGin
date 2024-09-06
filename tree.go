package gin

import (
	"bytes"
	"github.com/zhangweijie11/zGin/iinternal/bytesconv"
	"strings"
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
	indices   string        // 目录
	wildChild bool          // 是否正则匹配
	nType     nodeType      // 节点类型
	priority  uint32        // 优先级
	children  []*node       // 子节点
	handlers  HandlersChain // 处理器流
	fullPath  string        // 完整路由路径
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
