package src

import (
	"math"
	"syscall"
)

const (
	RAND_MAX = math.MaxInt32 //0x7fffffff

)

type (
	uint64_t = uint64
	int64_t = int64
	int16_t = int16
)

func memcmp(__s1, __s2 interface{}, __n size_t) int {
	if __n == 0 {
		return 0
	}

	s1 := string(__s1.([]byte)[0:__n])//不包括中文的处理
	s2 := string(__s2.([]byte)[0:__n])


	switch {
	case s1 == s2: return 0
	case s1 < s2: return -1
	case s1 > s2: return 1
	}

	return 0
}

func abort() {
	syscall.Kill(syscall.Getpid(), syscall.SIGABRT)
}

func raise(signal syscall.Signal) {
	syscall.Kill(syscall.Getpid(), signal)
}