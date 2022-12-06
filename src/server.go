package src

import (
	"fmt"
	"log"
	"time"
)

type robj = redisObject
type redisObject struct{
	otype uint//类型，有string、list、hash、set、zset
	encoding uint//底层实现的数据结构
	lru uint
	refcount int
	ptr interface{}
}

type redisDb struct{
	id int//数据库的id，从0-15
	keyspace *dict//数据库的键空间，保存这本数据库所有的键值对，key是string对象，val是string、list、hash、set、zset等redis对象
	expires *dict//key=>设置了过期时间的key，val=>过期的unix时间点；key指向的是键空间里的键对象（共享对象，节约内存）


}

type listNode struct {
	prev *listNode
	next *listNode
	val interface{}
}

type list struct {
 	head *listNode
	tail *listNode
	len uint64

 	dup func(ptr uintptr)
 	free func(ptr uintptr)
 	match func(ptr uintptr, key uintptr) int
}

func (l *list) append(node *listNode) {
	if l.head == nil {
		l.len = 0
		n := &listNode{}
		l.head = n
		l.tail = n
	}

	node.prev = l.tail
	l.tail.next = node
	l.tail = l.tail.next
	l.len++
}
func (l *list) get(node *listNode) {

}
func (l *list) removeNode(node *listNode) {
	if l.len == 0 {
		return
	}
	l.tail = l.tail.prev
	//delete(l.tail.next)
	l.tail.next = nil
	l.len--
}

var (
	server = &redisServer{}

	io_threads_op int = 0
)


type redisServer struct{//服务端
	db *redisDb//数组，保存着本服务实例中所有的数据库（一般是16个，从0-15），指向数据库数组的开端（即第一个数据库的地址）
	dbNum int//初始化时，限制的数据库最多个数，一般是16

	statKeyspaceHits   int //找到key的次数
	statKeyspaceMisses int //找不到key的次数

	/*发布订阅*/
	pubsubChannels *dict//保存所有频道的订阅关系，key=>频道名，val=>订阅的client链表，新订阅的client放在链表末尾
	pubsubPatterns *list//保存所有模式的订阅关系，list的node.val是pubsubPattern

	saveparams []saveparam//进行rdb文件生成的配置规则 数组，比如每60秒内有10次修改，就触发bgsave生成rdb备份文件
	dirty int64//上一次执行save后，client对数据库（全部数据库）进行了多少次更新操作（CUD），如set msg hi则dirty+1，sadd tsetoo 1 2 3则dirty+3
	lastsave int64//上一次执行save的时间戳

	aof_buf *sds //AOF buffer, 在进入event loop前 先写入到AOF buffer

	clients *list//所有与server连接的client，新连接的client添加到链表末尾
	clients_to_close *list//等待异步关闭的client链表

	/*lua*/
	luaScript *dict //lua脚本字典，key=>脚本的sha1校验和，val=>对应的脚本
	luaClient *client//负责执行lua脚本的伪客户端，该客户端直到server关闭时才会关闭
	repl_scriptcache_dict *dict
	//aofClient *client//负责在启动时执行aof的伪客户端，载入完毕后关闭

	masterhost string//masterIP地址
	masterport int//


	tcpkeepalive int               /* Set SO_KEEPALIVE if non-zero. */
	el *aeEventLoop//事件循环对象

	io_threads_num int         /* Number of IO threads to use. io线程个数*/
	io_threads_do_reads bool    /* Read and parse from IO threads? 正从io线程读取/解析数据吗 */
	io_threads_active bool      /* Is IO threads currently active? io线程当前是否激活*/

	clients_pending_read *list //等待连接read buffer的clients链表

	stat_total_reads_processed int64 /* Total number of read events processed */
	stat_total_writes_processed int64 /* Total number of write events processed */

	/*慢日志*/
	slowlog *list//慢日志链表 entry = slowlogEntry
	slowlog_entry_id int64 //下一条慢日志的id
	slowlog_log_slower_than int64 //执行时间>这个值，就会被存入慢日志
	slowlog_max_len int64 //配置的选项值，慢日志存储条数，大于这个值，根据FIFO，把旧的先删除再插入新日志
}


type saveparam struct{
	seconds time.Duration//秒数
	changes int//修改次数
}

func Main() {
	zmalloc_set_oom_handler(redisOutOfMemoryHandler)
}

func redisOutOfMemoryHandler(allocation_size size_t) {
	log.Println(fmt.Sprintf(LL_WARNING,"Out Of Memory allocating %zu bytes!", allocation_size))
	panic(fmt.Sprintf("Redis aborting for OUT OF MEMORY. Allocating %zu bytes!", allocation_size))
}