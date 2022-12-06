package src

type redisCommandProc func(c *client)
type redisGetKeysProc func(cmd *redisCommand, argv []*robj, argc int, result *getKeysResult) int

type keyReference struct {
	pos int /* The position of the key within the client array */
	flags int /* The flags associated with the key access, see CMD_KEY_* for more information */
}

type getKeysResult struct {
	keysbuf [MAX_KEYS_BUFFER]keyReference       /* 预分配buffer，保存堆数据 */
	keys *keyReference                          /* Key indices array, points to keysbuf or heap */
	numkeys int                        /* Number of key indices return */
	size int                           /* Available array size */
}

type redisCommandArg struct {
	name string
 	atype redisCommandArgType
	keySpecIndex int
	token string
	summary string
	since string
	flags int
	deprecatedSince string
	subargs *redisCommandArg
	/* runtime populated data */
	numArgs int
}

type redisCommand struct {
	name string//命令名字

	declaredName string//命令（和客户端输入的命令一样）
	summary string//命令的介绍
	complexity string//时间复杂度
	since string //引入命令时的版本 如2.6.0
	replacedBy string //旧命令弃用时，用该新命令替代
	proc redisCommandProc//命令的实现
	arity int//命令的参数个数，用-N 表示至少得>=N个参数
	flags uint64 //see CMD_ARG_NONE

	getkeys_proc redisGetKeysProc

	args []redisCommandArg
}

