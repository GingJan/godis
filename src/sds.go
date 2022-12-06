package src

import (
	"reflect"
	"unsafe"
)

const (
	SDS_TYPE_5  = 0
	SDS_TYPE_8  = 1
	SDS_TYPE_16 = 2
	SDS_TYPE_32 = 3
	SDS_TYPE_64 = 4
	SDS_TYPE_MASK = 7
	SDS_TYPE_BITS = 3
)

type sdshdr5 struct {
	len uint8
	free uint8
	flags byte /* 3 lsb of type, and 5 msb of string length，低3位存type类型，高5位存字符串长度 */
	buf []byte
}

type sdshdr8 struct {
	len uint8//字符串长度
	free uint8//空闲空间
	sdshdr5
}

type sdshdr16 struct {
	len uint16//字符串长度
	free uint16//空闲空间
	sdshdr5
}

type sdshdr32 struct {
	len uint32//字符串长度
	free uint32//空闲空间
	sdshdr5
}

type sdshdr64 struct {
	len uint64//字符串长度
	free uint64//空闲空间
	sdshdr5
}

func sdslen(s sds) size_t {
	flags := s.flags
	switch flags & SDS_TYPE_MASK {
	case SDS_TYPE_5:
		return size_t(flags>>SDS_TYPE_BITS)
	case SDS_TYPE_8:
		return size_t(s.len)
	case SDS_TYPE_16:
		return size_t(s.len)
	case SDS_TYPE_32:
		return size_t(s.len)
	case SDS_TYPE_64:
		return size_t(s.len)
	}

	return 0
}

func sdsHdrSize(ttype byte) int {
	switch ttype & SDS_TYPE_MASK {
	case SDS_TYPE_5:
		var sdshdr5 sdshdr5
		//var sdshdr5 = sdshdr5{} ??
		return int(unsafe.Sizeof(sdshdr5))
	case SDS_TYPE_8:
		var sdshdr8 sdshdr8
		return int(unsafe.Sizeof(sdshdr8))
	case SDS_TYPE_16:
		var sdshdr16 sdshdr16
		return int(unsafe.Sizeof(sdshdr16))
	case SDS_TYPE_32:
		var sdshdr32 sdshdr32
		return int(unsafe.Sizeof(sdshdr32))
	case SDS_TYPE_64:
		var sdshdr64 sdshdr64
		return int(unsafe.Sizeof(sdshdr64))
	}

	return 0
}

func sdsalloc(s sds) size_t {
	flags := s.flags
	switch flags & SDS_TYPE_MASK {
	case SDS_TYPE_5:
		return size_t(flags>>SDS_TYPE_BITS)
	case SDS_TYPE_8:
		return size_t(s.alloc)
	case SDS_TYPE_16:
		return size_t(s.alloc)
	case SDS_TYPE_32:
		return size_t(s.alloc)
	case SDS_TYPE_64:
		return size_t(s.alloc)
	}

	return 0
}

/* Free an sds string. No operation is performed if 's' is NULL. */
func sdsfree(s sds) {
	if s == nil {
		return
	}

	s_free(unsafe.Pointer(uintptr(unsafe.Pointer(s)) - uintptr(sdsHdrSize(s.flags))))
}

/* Compare two sds strings s1 and s2 with memcmp().
 *
 * Return value:
 *
 *     positive if s1 > s2.
 *     negative if s1 < s2.
 *     0 if s1 and s2 are exactly the same binary string.
 *
 * If two strings share exactly the same prefix, but one of the two has
 * additional characters, the longer string is considered to be greater than
 * the smaller one. */
func sdscmp(s1 sds, s2 sds) int {
	var l1, l2, minlen size_t
	var cmp int

	l1 = sdslen(s1)
	l2 = sdslen(s2)
	if l1 < l2 {
		minlen = l1
	} else {
		minlen = l2
	}

	if minlen == 0 {
		return 0
	}

	cmp = memcmp(s1.buf, s2.buf, minlen)
	if cmp != 0 {
		return cmp
	}
	switch {
		case l1 > l2: return 1
		case l1 < l2: return -1
		case l1 == l2: return 0
	}

}