package src

import "time"

/* Client MULTI/EXEC state */
type multiCmd struct {
	argv []*robj
	argvLen int
	argc int
 	cmd *redisCommand
}

type multiState struct {
	commands []multiCmd     /* 事务的命令队列 FIFO*/
	count int              /* 已入队的命令数 */
	cmdFlags int          /* The accumulated command flags OR-ed together.
   So if at least a command has a given flag, it
   will be set in this field. */
	cmdInvFlags int      /* Same as cmd_flags, OR-ing the ~flags. so that it
   is possible to know if all the commands have a
   certain flag. */
	argvLenSums size_t    /* mem used by all commands arguments */
}

type client = redisClient
type redisClient struct{//客户端
	name *robj//客户端的名字，一般都是没有的，可使用 client setname xxx命令设置
	fd int //和客户端连接成功后，accept返回的fd，为客户端的fd=-1（用于在启动server时，从AOF读取命令或执行Lua脚本使用，不是来自网络的fd）
	querybuf sds //client的输入缓冲区，如发送了set key value命令，则这里就是一个sds，含有 *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n，该sds是动态扩缩，但不可超过1G，否则server会把该client关闭
	qb_pos size_t          /* The position we have read in querybuf. */
	querybuf_peak size_t //当前querybuf已用空间

	db *redisDb //当前客户端使用的数据库（从0-15）

	argv []*robj//命令的参数，server把querybuf的输入解析后，放入该字段，argv[0]是命令，其他是参数
	argc int//记录argv数组的元素个数，如上set key value命令，这里=3（包含命令也算）

	cmd *redisCommand//根据输入的命令，找到该命令的handler（命令执行逻辑/函数）

	flags int //see CLIENT_SLAVE，标示当前client不同的角色和当前状态，使用 或| 组成

	//返回缓冲 固定大小
	buf [PROTO_REPLY_CHUNK_BYTES]byte//server每次返回给client的数据，固定大小，用于存放返回值较小的数据，比如OK，错误回复，简短字符串，整数等
	bufpos int//记录buf已用byte的数量
	//返回缓冲 可变大小（当返回的数据太大或buf用完空间时，则使用reply）
	reply *list//链表方式扩展，无限大小

	reqtype int //请求类型，1普通命令，2multi事务命令
	authenticated int//是否通过身份验证，0未通过，1通过，AUTH命令

	ctime time.Duration//与服务端已建立连接时长，秒
	lastinteraction int64 //与服务端最后一次交互的时间戳
	obufSoftLimitReachedTime int64//第一次达到超出缓冲区的软性限制，若在规定时间（该时间可配置）内持续超过软性限制，则client会被server关闭连接

	/*事务*/
	mstate multiState /* MULTI/EXEC state */

	pending_read_list_node *listNode
}

func createClient(conn *connection) *client {
	c := new(client)

	if conn != nil {
		connEnableTcpNoDelay(conn)
		if server.tcpkeepalive != 0 {

		}
	}
}

func connEnableTcpNoDelay(conn *connection) int {
	if conn.fd == -1 {
		return C_ERR
	}

	return anetEnableTcpNoDelay(nil, conn.fd)
}

func connKeepAlive(conn *connection, interval int) int {
	if conn.fd == -1 {
		return C_ERR
	}

	return anetKeepAlive(nil, conn.fd, interval)
}
