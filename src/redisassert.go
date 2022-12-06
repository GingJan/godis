package src

import (
	"fmt"
	"os"
	"syscall"
)

func _serverAssertInRedisassert(estr string , file string, line int) {
	fmt.Fprintf(os.Stderr, "=== ASSERTION FAILED ===")
	fmt.Fprintf(os.Stderr, "==> %s:%d '%s' is not true",file,line,estr)
	raise(syscall.SIGSEGV)
}

func _serverPanicInRedisassert(file string, line int, msg string, a... interface{}) {
	fmt.Fprintf(os.Stderr, "------------------------------------------------")
	fmt.Fprintf(os.Stderr, "!!! Software Failure. Press left mouse button to continue")
	fmt.Fprintf(os.Stderr, "Guru Meditation: %s #%s:%d",msg,file,line)
	abort()
}