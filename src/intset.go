package src

const (
	INTSET_ENC_INT16 = 1//contents 每个元素是int16的值
	INTSET_ENC_INT32 = 2//contents 每个元素是int32的值
	INTSET_ENC_INT64 = 3//contents 每个元素是int64的值
)
type intset struct {
	encoding uint32//编码方式
	length uint32//元素个数
	contents []int8//包含的元素
}

func (i *intset) Add(num interface{}) {
	switch num.(type) {
	case int16:

	case int32:
	case int64:
	default:
		return
	}
}


