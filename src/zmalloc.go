package src

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var PREFIX_SIZE uint
func init() {
	var sizet size_t
	PREFIX_SIZE = uint(unsafe.Sizeof(sizet))
}

var zmalloc_oom_handler = zmalloc_default_oom

func zmalloc_set_oom_handler(oom_handler func(size size_t)) {
	zmalloc_oom_handler = oom_handler
}

func MALLOC_MIN_SIZE(x size_t) size_t {
	if x > 0 {
		return x
	}
	return unsafe.Sizeof(lONG)
}

func zfree(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	//统计ptr被回收后，释放的空间
}

/* Allocate memory or panic */
func zmalloc(size size_t) unsafe.Pointer {
	ptr := ztrymalloc_usable(size, nil)
	if ptr == nil {
		zmalloc_oom_handler(size)
	}
	return ptr
}

func zmalloc_default_oom(size size_t) {
	fmt.Fprintf(os.Stdout, "zmalloc: Out of memory trying to allocate %du bytes\n", size)
	abort()
}


static void (*zmalloc_oom_handler)(size_t) = zmalloc_default_oom;

/* Try allocating memory, and return NULL if failed.
 * '*usable' is set to the usable size if non NULL. */
func ztrymalloc_usable(size size_t, usable *size_t) unsafe.Pointer {
	//ASSERT_NO_SIZE_OVERFLOW(size)
	//ptr := malloc(MALLOC_MIN_SIZE(size) + PREFIX_SIZE)
	//
	//if (!ptr) return NULL;
	//#ifdef HAVE_MALLOC_SIZE
	//size = zmalloc_size(ptr);
	//update_zmalloc_stat_alloc(size);
	//if (usable) *usable = size;
	//return ptr;
	//#else
	//*((size_t*)ptr) = size;
	//update_zmalloc_stat_alloc(size+PREFIX_SIZE);
	//if (usable) *usable = size;
	//return (char*)ptr+PREFIX_SIZE;
	//#endif
}

/* Allocate memory and zero it or panic */
func zcalloc(size size_t) unsafe.Pointer {
	ptr := ztrycalloc_usable(size, nil)
	if ptr == nil {
		zmalloc_oom_handler(size)
	}
	return ptr
}

/* Try allocating memory, and return NULL if failed. */
func ztrycalloc(size size_t) unsafe.Pointer {
	return ztrycalloc_usable(size, nil)
}

/* Try allocating memory and zero it, and return NULL if failed.
 * '*usable' is set to the usable size if non NULL. */
func ztrycalloc_usable(size size_t, usable *size_t) unsafe.Pointer {
	ASSERT_NO_SIZE_OVERFLOW(size)
	ptr := calloc(1, MALLOC_MIN_SIZE(size)+PREFIX_SIZE)
	if ptr == nil {
		return nil
	}
	size = zmalloc_size(ptr)
	update_zmalloc_stat_alloc(size)
	if usable != nil {
		*usable = size
	}
	return ptr
}

func ASSERT_NO_SIZE_OVERFLOW(sz ) {
	assert((sz) + PREFIX_SIZE > (sz), "(sz) + PREFIX_SIZE > (sz)")
}