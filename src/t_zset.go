package src

import (
	"math"
	"math/rand"
	"unsafe"
)

/* 用指定的level层级数来新建一个skiplist的结点
 * SDS类型的字符串 ele 是结点存储的值 */
func zslCreateNode(level int, score float64, ele sds) *zskiplistNode {
	var zn *zskiplistNode = new(zskiplistNode)
	zl := zskiplistLevel{}
	zmalloc(uint(unsafe.Sizeof(zn) + uintptr(level) * unsafe.Sizeof(zl)))
	zn.score = score
	zn.ele = ele
	return zn
}

/* Create a new skiplist. 创建一个跳表*/
func zslCreate() *zskiplist {
	var j int

	var zsl *zskiplist = new(zskiplist)
	zsl.level = 1
	zsl.length = 0

	//先创建32层的（虚）头结点并初始化
	zsl.header = zslCreateNode(ZSKIPLIST_MAXLEVEL,0, nil)
	for j = 0; j < ZSKIPLIST_MAXLEVEL; j++ {
		zsl.header.level[j].forward = nil
		zsl.header.level[j].span = 0
	}

	zsl.header.backward = nil
	zsl.tail = nil

	return zsl
}

/* Free the specified skiplist node. The referenced SDS string representation
 * of the element is freed too, unless node->ele is set to NULL before calling
 * this function.
释放指定结点，代表元素的SDS字符串也同时被释放，除非在调用该函数前node.ele被设为nil
*/
func zslFreeNode(node *zskiplistNode) {
	sdsfree(node.ele)
	zfree(unsafe.Pointer(node))
}

/* 删除/释放整个跳表 */
func zslFree(zsl *zskiplist) {
	var node *zskiplistNode = zsl.header.level[0].forward
	var next *zskiplistNode

	zfree(unsafe.Pointer(zsl.header))
	for node != nil {
		next = node.level[0].forward
		zslFreeNode(node)
		node = next
	}

	zfree(unsafe.Pointer(zsl))
}

/* Returns a random level for the new skiplist node we are going to create.
 * The return value of this function is between 1 and ZSKIPLIST_MAXLEVEL
 * (both inclusive), with a powerlaw-alike distribution where higher
 * levels are less likely to be returned. */
func zslRandomLevel() int {
	threshold := ZSKIPLIST_P*RAND_MAX
	level := 1//默认至少1层
	for float64(rand.Int31()) < threshold {//TODO 等待一致
		level += 1//25%的概率增加1层
	}

	if level < ZSKIPLIST_MAXLEVEL {
		return level
	}

	return ZSKIPLIST_MAXLEVEL
}

/* Insert a new node in the skiplist. Assumes the element does not already
 * exist (up to the caller to enforce that). The skiplist takes ownership
 * of the passed SDS string 'ele'.
往跳表里插入一个新结点，先假设该结点的元素ele在跳表里不存在（取决于调用方是否强行认为）
*/
func zslInsert(zsl *zskiplist, score float64, ele sds) *zskiplistNode {
	var update [ZSKIPLIST_MAXLEVEL]*zskiplistNode//新score&ele结点在每层要插入的位置 的前一个结点
	var rank [ZSKIPLIST_MAXLEVEL]uint64//[层级]rank即span跨度，rank是排名，在寻找某个结点时，由经过结点的span值相加得出

	if !math.IsNaN(score) {//插入的score非数值
		serverAssert(false)
	}

	curr := zsl.header//x从header的下一个结点开始遍历
	for lvl := zsl.level - 1; lvl >= 0; lvl-- {//lvl从最高层开始往下遍历，寻找新结点在每层要插入的位置
		/* store rank that is crossed to reach the insert position */
		if lvl == zsl.level - 1 {
			rank[lvl] = 0
		} else {
			rank[lvl] = rank[lvl+1]
		}

		//寻找新 score&ele 要插入的位置
		for curr.level[lvl].forward != nil {
			//每层结点找到插入位置前一结点（score刚好比前结点大 或 score相等下 ele字典序比前结点大）
			if curr.level[lvl].forward.score < score || (curr.level[lvl].forward.score == score && sdscmp(curr.level[lvl].forward.ele, ele) < 0) {
				rank[lvl] += curr.level[lvl].span//累计在本lvl层（本层）遍历过的结点的 总跨度，下一lvl层以此为起点继续累计
				curr = curr.level[lvl].forward//curr指针移动到 本lvl层（本层）的下一个结点，继续遍历
			}
			//else{否则，新结点在lvl层找到要插入的位置了}
		}

		update[lvl] = curr//update存着新结点要插入位置的前一结点（即在第lvl层的x结点后插入新结点）
		//第lvl层遍历完毕并已找到新结点的插入位置，继续遍历下一层，此时不会从下一层的头结点开始，而是从本层的当前curr结点开始
	}
	//至此，rank[lvl]代表在lvl层，从头结点到插入位置前一节点（含）这段的总span跨度（即跨了多少个处于第1层的结点）
	// update[lvl]代表新结点在各lvl层的插入位置的前一节点

	/* 先假设元素不在里面，因为允许存在相同score的原因，
	不应重复插入相同元素，因为本函数的调用者应在hash table里检测要插入的元素是否已存在
	we assume the element is not already inside, since we allow duplicated
	 * scores, reinserting the same element should never happen since the
	 * caller of zslInsert() should test in the hash table if the element is
	 * already inside or not. */
	level := zslRandomLevel()//该新结点拥有的level层级
	if level > zsl.level {//若新结点的level层级比现有全部结点的level都高
		//先对新层级进行初始化
		for lvl := zsl.level; lvl < level; lvl++ {
			rank[lvl] = 0
			update[lvl] = zsl.header
			update[lvl].level[lvl].span = zsl.length//先假设新层级的span跨度=整个链表长度
		}

		zsl.level = level//更新整个链表的最高层级
	}

	newNode := zslCreateNode(level,score,ele)//创建一个新结点实体
	for lvl := 0; lvl < level; lvl++ {//更新新结点在各层的forward、span值，以及在各层的前一节点的forward、span
		newNode.level[lvl].forward = update[lvl].level[lvl].forward//新结点在lvl层的forward指向下一结点 = 插入位置前一个结点在lvl层的下一结点
		update[lvl].level[lvl].forward = newNode//插入位置的前一个结点的forward指向新结点

		/* update span covered by update[lvl] as x is inserted here */
		//根据上面的分析，rank[0]值是最大的，而rank[lvl]是在第lvl层的从头节点到当前插入位置前一结点的跨度，rank[0]-rank[lvl]代表往后直到尾结点剩余的跨度
		newNode.level[lvl].span = update[lvl].level[lvl].span - (rank[0] - rank[lvl])//更新 新结点在本lvl层级的跨度span
		update[lvl].level[lvl].span = (rank[0] - rank[lvl]) + 1
	}

	/* increment span for untouched levels */
	for lvl := level; lvl < zsl.level; lvl++ {
		update[lvl].level[lvl].span++
	}

	//设置新结点的backward
	if update[0] == zsl.header {
		newNode.backward = nil
	} else {
		newNode.backward = update[0]
	}
	//设置新结点的后一结点的backward
	if newNode.level[0].forward != nil {
		newNode.level[0].forward.backward = newNode
	} else {
		zsl.tail = newNode
	}

	zsl.length++
	return newNode
}

/* Internal function used by zslDelete, zslDeleteRangeByScore and
 * zslDeleteRangeByRank. */
func zslDeleteNode(zsl *zskiplist, x *zskiplistNode, update *[ZSKIPLIST_MAXLEVEL]*zskiplistNode) {
	for i := 0; i < zsl.level; i++ {//寻找要删的目标结点x
		if (*update)[i].level[i].forward == x {//找到了，则把x从跳表里删除
			(*update)[i].level[i].span += x.level[i].span - 1
			(*update)[i].level[i].forward = x.level[i].forward
		} else {//在本层没有x结点，则span自动-1（因为基层必定会减少一个结点）继续下一lvl层的寻找
			(*update)[i].level[i].span -= 1
		}
	}

	//设置被删结点的后一节点的新forward指向
	if x.level[0].forward != nil {
		x.level[0].forward.backward = x.backward
	} else {
		zsl.tail = x.backward
	}

	for zsl.level > 1 && zsl.header.level[zsl.level-1].forward == nil {
		zsl.level--
	}

	zsl.length--
}

/* Delete an element with matching score/element from the skiplist.
 * The function returns 1 if the node was found and deleted, otherwise
 * 0 is returned.
 *
 * If 'node' is NULL the deleted node is freed by zslFreeNode(), otherwise
 * it is not freed (but just unlinked) and *node is set to the node pointer,
 * so that it is possible for the caller to reuse the node (including the
 * referenced SDS string at node->ele).
从跳表里删除指定的score和ele元素，返回1代表删除成功，0代表没有该结点
如果node是nil，结点会被zslFreeNode()释放内存，否则被删除结点的内存不会被释放，只是从跳表里移除。
*/
func zslDelete(zsl *zskiplist, score float64, ele sds, node **zskiplistNode) int {
	var update [ZSKIPLIST_MAXLEVEL]*zskiplistNode
	var x *zskiplistNode

	x = zsl.header//从头结点的下一个结点 开始遍历
	for lvl := zsl.level-1; lvl >= 0; lvl-- {
		for x.level[lvl].forward != nil && (x.level[lvl].forward.score < score ||
				(x.level[lvl].forward.score == score && sdscmp(x.level[lvl].forward.ele,ele) < 0)) {
			x = x.level[lvl].forward//x移向下一个结点继续遍历
		}

		update[lvl] = x//在第i层停留的结点
		//本lvl层遍历完毕，开始下一层的遍历
	}

	/* 可能会有多个元素的score是相同的，但我们只删除score和ele一致的结点 */
	x = x.level[0].forward
	if x != nil && score == x.score && sdscmp(x.ele,ele) == 0 {
		zslDeleteNode(zsl, x, &update)//从跳表删除score&ele对应的结点

		if node == nil {
			zslFreeNode(x)//释放给被删结点的内存
		} else {
			*node = x//被删结点返回给调用者使用
		}

		return 1
	}

	return 0 /* not found */
}

/* Update the score of an element inside the sorted set skiplist.
 * Note that the element must exist and must match 'score'.
 * This function does not update the score in the hash table side, the
 * caller should take care of it.
 *
 * Note that this function attempts to just update the node, in case after
 * the score update, the node would be exactly at the same position.
 * Otherwise the skiplist is modified by removing and re-adding a new
 * element, which is more costly.
 *
 * The function returns the updated element skiplist node pointer. */
func zslUpdateScore(zsl *zskiplist, curscore float64, ele sds, newscore float64) *zskiplistNode {
	var update [ZSKIPLIST_MAXLEVEL]*zskiplistNode
	var x *zskiplistNode

	/* We need to seek to element to update to start: this is useful anyway,
	 * we'll have to update or remove it. */
	x = zsl.header
	for lvl := zsl.level-1; lvl >= 0; lvl-- {
		for x.level[lvl].forward != nil && (x.level[lvl].forward.score < curscore ||
			(x.level[lvl].forward.score == curscore && sdscmp(x.level[lvl].forward.ele,ele) < 0)) {
			x = x.level[lvl].forward
		}

		update[lvl] = x
	}

	/* x指针移动到定位到的结点，注意本函数是假设能找到指定score的元素 */
	x = x.level[0].forward
	serverAssert(x != nil && curscore == x.score && sdscmp(x.ele,ele) == 0)

	/* 如果score更新后，结点还是在原位，则只更新score，无需删除它然后再重新插入到新位置 */
	if (x.backward == nil || x.backward.score < newscore) &&
	(x.level[0].forward == nil || x.level[0].forward.score > newscore) {
		x.score = newscore
		return x//返回新score的原结点
	}

	/* 删除原结点，并插入新结点（含新score）到新的位置（因为score的变更改变了结点的位置） */
	zslDeleteNode(zsl, x, &update)

	newnode := zslInsert(zsl,newscore,x.ele)
	/* 复用x.ele对应的sds字符串，只释放结点的内存，因为zslInsert函数会创建一个新结点 */
	x.ele = nil
	zslFreeNode(x)

	return newnode//返回新score的新结点
}

func zslValueGteMin(value float64, spec *zrangespec) bool {
	if spec.minex {//左边是开区间
		return value > spec.min
	}
	return value >= spec.min
}

func zslValueLteMax(value float64, spec *zrangespec) bool {
	if spec.maxex {//右边是闭区间
		return value < spec.max
	}
	return value <= spec.max
}

/* zset里是否含有指定score范围的值 */
func zslIsInRange(zsl *zskiplist, rangei *zrangespec) bool {
	var x *zskiplistNode

	/* 参数合法性检测 */
	if rangei.min > rangei.max ||
	(rangei.min == rangei.max && (rangei.minex || rangei.maxex)) {
		return false
	}

	x = zsl.tail
	if x == nil || !zslValueGteMin(x.score,rangei) {//空跳表或不在范围（左）
		return false
	}

	x = zsl.header.level[0].forward
	if x == nil || !zslValueLteMax(x.score,rangei) {//空跳表或不在范围（右）
		return false
	}

	return true
}

/* 返回指定范围内找到的第一个结点，如果指定范围内找不到结点，则返回nil */
func zslFirstInRange(zsl *zskiplist, rangei *zrangespec) *zskiplistNode {
	var x *zskiplistNode

	/* If everything is out of range, return early. 如果指定范围不存在，则提前return */
	if !zslIsInRange(zsl,rangei) {
		return nil
	}

	x = zsl.header
	for lvl := zsl.level-1; lvl >= 0; lvl-- {
		/* 如果不在范围内，一直遍历 */
		for x.level[lvl].forward != nil && !zslValueGteMin(x.level[lvl].forward.score, rangei) {
			x = x.level[lvl].forward
		}
	}

	/* This is an inner rangei, so the next node cannot be NULL. */
	x = x.level[0].forward
	serverAssert(x != nil)

	/* Check if score <= max. */
	if !zslValueLteMax(x.score,rangei) {
		return nil
	}

	return x
}

/* 返回指定范围内找到的最后一个结点，如果指定范围内找不到结点，则返回nil */
func zslLastInRange(zsl *zskiplist, rangei *zrangespec) *zskiplistNode {
	var x *zskiplistNode

	/* If everything is out of rangei, return early. */
	if !zslIsInRange(zsl,rangei) {
		return nil
	}

	x = zsl.header
	for lvl := zsl.level-1; lvl >= 0; lvl-- {
		/* 如果在范围内，继续遍历，找到找到最后一个 */
		for x.level[lvl].forward != nil && zslValueLteMax(x.level[lvl].forward.score, rangei) {
			x = x.level[lvl].forward
		}
	}

	/* This is an inner rangei, so this node cannot be NULL. */
	serverAssert(x != nil)

	/* Check if score >= min. */
	if !zslValueGteMin(x.score,rangei) {
		return nil
	}

	return x
}

/* 从跳表里删除score从min到max范围内的元素。开闭区间取决于rangi参数
 * Note that this function takes the reference to the hash table view of the
 * sorted set, in order to remove the elements from the hash table too. */
func zslDeleteRangeByScore(zsl *zskiplist, rangei *zrangespec, dict *dict) uint64 {
	var update [ZSKIPLIST_MAXLEVEL]*zskiplistNode

	x := zsl.header
	for lvl := zsl.level-1; lvl >= 0; lvl-- {
		for x.level[lvl].forward != nil && !zslValueGteMin(x.level[lvl].forward.score, rangei) {
			x = x.level[lvl].forward
		}

		update[lvl] = x
	}

	var removed uint64 = 0//被删除的结点个数
	/* 当前x结点是最后一个<或<=min的结点，所以再往前取一个，则>min */
	x = x.level[0].forward
	//此时 x 为范围内的左边界结点
	/* Delete nodes while in range. */
	for x != nil && zslValueLteMax(x.score, rangei) {//x非空且x.score未到右边界
		next := x.level[0].forward
		zslDeleteNode(zsl,x, &update)
		dictDelete(dict,x.ele)
		zslFreeNode(x) /* x.ele 元素真正释放的地方 */
		removed++
		x = next
	}

	return removed
}