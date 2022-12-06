package src

/* This structure defines an entry inside the slow log list */
type slowlogEntry struct {
	argv []*robj//命令和参数
	argc int
	id int64       /* Unique entry identifier. */
	duration int64 /* Time spent by the query, in microseconds. 命令耗时，微秒*/
	time time_t         /* Unix time at which the query was executed. 该慢命令执行的时间戳秒*/
	cname sds          /* Client name. */
	peerid sds         /* Client network address. */
}

/* Exported API */
func slowlogInit(){}
func slowlogPushEntryIfNeeded(c *client, argv []*robj, argc int, duration int64){}
