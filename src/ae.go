package src

const (
	AE_READABLE = 1
	AE_WRITABLE = 2
	AE_BARRIER = 3
)

const (
	AE_TIME_EVENTS = 1<<1 //2
	AE_DONT_WAIT = 1<<2 //4
)
type aeFileProc func(eventLoop *aeEventLoop, fd int, clientData uintptr, mask int)
type aeTimeProc func(eventLoop *aeEventLoop, id int64, clientData uintptr) int
type aeEventFinalizerProc func(eventLoop *aeEventLoop, clientData uintptr)
type aeBeforeSleepProc func(eventLoop *aeEventLoop)
/* File event structure */
type aeFileEvent struct {
	mask int /* one of AE_(READABLE|WRITABLE|BARRIER) */
	rfileProc aeFileProc//可读事件（文件事件）处理函数handler
	wfileProc aeFileProc//可写事件（文件事件）处理函数handler
	clientData uintptr//or使用interface？事件待处理的数据
}
type monotime uint64

/* Time event structure */
type aeTimeEvent struct {
	id int64 /* time event identifier. */
	when monotime
	timeProc aeTimeProc
	finalizerProc aeEventFinalizerProc
	clientData uintptr
	prev *aeTimeEvent
	next *aeTimeEvent
	refcount int /* refcount to prevent timer events from being freed in recursive time event calls. */
}

/* A fired event */
type aeFiredEvent struct {
	fd int
	mask int
}

/* State of an event based program */
type aeEventLoop struct {
	maxfd int   /* 已注册但未就绪的文件事件个数（最大的那个文件fd） */
	setsize int /* max number of file descriptors tracked */
	timeEventNextId int64
	events []aeFileEvent /* Registered events 未就绪文件事件数组 */
	fired []aeFiredEvent /* Fired events，就绪文件事件数组，当epoll监听的socket有事件触发时，则会把epoll返回的rdlist里的socket对应的事件放入本字段 */
	timeEventHead *aeTimeEvent// 时间事件链表
	stop int
	apidata uintptr /* epoll This is used for polling API specific data */
	beforesleep aeBeforeSleepProc
	aftersleep aeBeforeSleepProc
	flags int
}

func aeProcessEvents(eventLoop *aeEventLoop, flags int) int {
	var processed int = 0
	var numevents int

	if eventLoop.maxfd != -1 || ((flags & AE_TIME_EVENTS) && !(flags & AE_DONT_WAIT)) {
		
	}


}