package bytesconv

import "unsafe"

// StringToBytes 将字符串转换为字节切片
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString 将字节切片转换为字符串
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
