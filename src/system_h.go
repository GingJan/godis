package src

import (
	"fmt"
	"math"
	"math/rand"
	"syscall"
	"time"
	"unsafe"
)

var (
	lONGMAX int
	lONG    int64
)

const (
	RAND_MAX  = math.MaxInt32 //0x7fffffff
	ULONG_MAX = RAND_MAX
	CHAR_BIT  = 8
	LONG_MAX  = unsafe.Sizeof(lONGMAX)
)

type (
	uint8_t  = uint8
	uint64_t = uint64
	int64_t  = int64
	int16_t  = int16
)

func memcmp(__s1, __s2 interface{}, __n size_t) int {
	if __n == 0 {
		return 0
	}

	s1 := string(__s1.([]byte)[0:__n]) //不包括中文的处理
	s2 := string(__s2.([]byte)[0:__n])

	switch {
	case s1 == s2:
		return 0
	case s1 < s2:
		return -1
	case s1 > s2:
		return 1
	}

	return 0
}

func abort() {
	syscall.Kill(syscall.Getpid(), syscall.SIGABRT)
}

func raise(signal syscall.Signal) {
	syscall.Kill(syscall.Getpid(), signal)
}

func random() int {
	return rand.Int()
}

type timeval = time.Time

//type timeval struct {
//	tv_sec int64 //秒
//	tv_usec int32 //微秒
//}
func gettimeofday(time2 *time.Time, p unsafe.Pointer) {
	t := time.Now()
	time2 = &t
}

func memcpy(dest, src unsafe.Pointer, size uint) unsafe.Pointer {
	var d []byte
	r := (*[]byte)(src)
	copy(d[0:], (*r)[0:size])
	return unsafe.Pointer(&d)
}

func snprintf(buf *[]byte, size size_t, format string, a ...interface{}) size_t { //TODO
	output := []byte(fmt.Sprintf(format, a...))
	if size > size_t(len(output)) {
		size = size_t(len(output))
	}

	lenBuf := len(*buf)
	var i size_t
	for i = 0; i < size; i++ {
		(*buf)[lenBuf+1] = output[i]
	}

	//b := make([]byte, len(output))
	//r := strings.NewReader(output)
	//n, _ := r.Read(*buf)
	n := size
	return size_t(n)
}
