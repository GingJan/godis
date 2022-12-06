package src

const (
	ZSKIPLIST_MAXLEVEL = 32 /* Should be enough for 2^64 elements */
	ZSKIPLIST_P = 0.25      /* Skiplist P = 1/4 */
)

func serverAssert(_e bool) {
	if _e {
		return
	}

	_serverAssert("_e", "file", 0)
}

type zskiplistNode struct {//跳表结点
	ele sds
	score float64
	backward *zskiplistNode
	level [...]zskiplistLevel
}
type zskiplistLevel struct {//跳表结点层级
	forward *zskiplistNode//下一个结点
	span uint64//跨度，本结点跨span个结点到达下一个结点（都是指同level层级的）
}

type zskiplist struct {//整个跳表
	header *zskiplistNode
	tail *zskiplistNode
	length uint64//跳表长度
	level int//以level最高的结点为本跳表的level值
}

/* Struct to hold an inclusive/exclusive range spec by score comparison. */
type zrangespec struct {
	min, max float64
	minex, maxex bool /* 是否包含min或max（开闭区间） */
}