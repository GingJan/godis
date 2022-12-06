package src

type zset struct {
	dict *dict
	zsl *zskiplist
}

func NewZskiplist() *zskiplist {
	headNode := &zskiplistNode{
		level: make([]*zskiplistLevel, 0, 32),
	}
	return &zskiplist{
		header: headNode,
		tail:   nil,
		length: 0,
		level:  0,
	}
}

func (z *zskiplist) zslInsert(score float64, obj *sdshdr) {

}
