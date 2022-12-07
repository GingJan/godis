package src

import "runtime"

func assert(_e bool, estr string) {
	if _e {
		return
	}
	_, file, line, _ := runtime.Caller(1) //TODO 获取调用assert的调用者file和line
	_serverAssertInRedisassert(estr, file, line)
	abort()
	return
}
