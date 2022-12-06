package src

import "unsafe"

func s_free (ptr unsafe.Pointer) {
	zfree(ptr)
}
