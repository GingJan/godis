package src

/* redisCommandArgType */
type redisCommandArgType int
const (
	ARG_TYPE_STRING redisCommandArgType = 1
	ARG_TYPE_INTEGER redisCommandArgType = 2
	ARG_TYPE_DOUBLE redisCommandArgType = 3
	ARG_TYPE_KEY redisCommandArgType = 4 /* A string, but represents a keyname */
	ARG_TYPE_PATTERN redisCommandArgType = 5
	ARG_TYPE_UNIX_TIME redisCommandArgType = 6
	ARG_TYPE_PURE_TOKEN redisCommandArgType = 7
	ARG_TYPE_ONEOF redisCommandArgType = 8 /* Has subargs */
	ARG_TYPE_BLOCK redisCommandArgType = 9 /* Has subargs */
)

type connectionState int
const (
	CONN_STATE_NONE connectionState = 0
	CONN_STATE_CONNECTING connectionState = 1
	CONN_STATE_ACCEPTING connectionState = 2
	CONN_STATE_CONNECTED connectionState = 3
	CONN_STATE_CLOSED connectionState = 4
	CONN_STATE_ERROR connectionState = 5
)
