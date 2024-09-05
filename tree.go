package gin

type Param struct {
	Key   string
	Value string
}

type Params []Param

type nodeType uint8

type node struct {
	path      string
	indices   string
	wildChild bool
	nType     nodeType
	priority  uint32
	children  []*node
	handlers  HandlersChain
	fullPath  string
}

type skippedNode struct {
	path        string
	node        *node
	paramsCount int16
}
