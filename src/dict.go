package src
//字典结构
import (
	"math"
	"runtime"
	"unsafe"
)

const (
	runningCmd = 0
	BG_SAVE = 1
	BG_REWRITE_AOF = 2

	ElasticNone = 0
	ElasticIncr = 1
	ElasticDecr = 2
)


type dictht struct {
	table []*dictEntry
	size uint64
	sizemask uint64
	used uint64
}

func (d *dict) NewDict() {
	initSize := uint64(4)
	dictht0 := &dictht{
		table:    make([]*dictEntry, 0, initSize),
		size:     initSize,
		sizemask: initSize - 1,
		used:     0,
	}
	dictht1 := &dictht{
		table:    make([]*dictEntry, 0, initSize),
		size:     initSize,
		sizemask: initSize - 1,
		used:     0,
	}
	dict := new(dict)
	dict.ht = [2]*dictht{dictht0,dictht1}
	dict.rehashidx = -1
}
func (d *dict) Put(key string, val interface{}) {
	d.checkElastic()
	dictEntry := &dictEntry{
		key:  key,
		val:  val,
		next: nil,
	}

	htNum, idx := d.getHashIdx(key)
	d.addEntry(htNum, idx, dictEntry)
}
func (d *dict) Delete(key string) {
	d.checkElastic()
	htNum, idx := d.getHashIdx(key)
	d.delEntry(htNum, idx, key)
}
func (d *dict) Get(key string) interface{} {
	d.checkElastic()
	htNum, idx := d.getHashIdx(key)
	entry := d.getEntry(htNum, idx, key)
	return entry.val
}
func (d *dict) Update(key string, val interface{}) bool {
	d.checkElastic()
	htNum, idx := d.getHashIdx(key)
	return d.updateEntry(htNum, idx, key, val)
}

func (d *dict) getHashIdx(key string) (int, uint64) {
	hash := d.dtype.hashFunction(key)
	htNum := 0
	if d.rehashidx > -1 {//正在进行rehash，所以ht[0]只减不增
		htNum = 1
	}

	return htNum, uint64(hash) & d.ht[htNum].sizemask
}

func (d *dict) addEntry(htNum int, idx uint64, dictEntry *dictEntry) {
	if d.ht[htNum] == nil {
		panic("warning fatal error occurred!")
	}

	if d.ht[htNum].table[idx] != nil {
		dictEntry.next = d.ht[htNum].table[idx]
	}
	d.ht[htNum].table[idx] = dictEntry
	d.ht[htNum].used++
}
func (d *dict) delEntry(htNum int, idx uint64, key string) {
	curr := d.ht[htNum].table[idx]
	for {
		if curr == nil {
			return
		}
		if curr.key == key {
			d.ht[htNum].table[idx] = curr.next
			d.ht[htNum].used--
			//delete(curr)//释放curr
			return
		}
		curr = curr.next
	}
}
func (d *dict) getEntry(htNum int, idx uint64, key string) *dictEntry {
	curr := d.ht[htNum].table[idx]
	for {
		if curr == nil {
			return nil
		}
		if curr.key == key {
			return curr
		}

		curr = curr.next
	}
}
func (d *dict) updateEntry(htNum int, idx uint64, key string, val interface{}) bool {
	entry := d.getEntry(htNum, idx, key)
	if entry == nil {
		return false
	}

	entry.val = val
	return true
}

//扩容
func (d *dict) checkElastic() {
	switch d.needElastic() {
	case ElasticIncr:
		d.doSizeIncr()
	case ElasticDecr:
		d.doSizeDecr()
	}
}
func (d *dict) doSizeIncr() {
	if d.rehashidx == -1 {//本轮第一次扩容
		d.rehashidx = 0
		incrToSize := uint64(math.Pow(float64(2), math.Log2(float64(d.ht[0].used) * 2) + 1))//2^n次方
		d.ht[1] = &dictht{
			table:    make([]*dictEntry, 0, incrToSize),
			size:     incrToSize,
			sizemask: incrToSize - 1,
			used:     0,
		}
	}

	for {
		curr := d.ht[0].table[d.rehashidx]
		if curr == nil {
			d.rehashidx++
			if d.rehashidx >= len(d.ht[0].table) {//完成扩容
				d.rehashidx = -1
				d.ht[0] = d.ht[1]
				d.ht[1] = nil
				return
			}

			continue
		} else {
			_, newIdx := d.getHashIdx(curr.key)
			d.ht[1].table[newIdx] = curr
			d.ht[1].used++
			d.ht[0].table[d.rehashidx] = curr.next//移动到下一个
			//delete(curr)//释放当前curr
			d.ht[0].used--

			return//每次操作只进行一次弹性扩容/缩小
		}
	}
}
func (d *dict) doSizeDecr() {}
func (d *dict) needElastic() int8 {
	loadFactor := float64(d.ht[0].used) / float64(d.ht[0].size)
	switch runningCmd {
	case BG_SAVE, BG_REWRITE_AOF:
		return d.elasticThreshold(loadFactor, 5, 0.1)
	default:
		return d.elasticThreshold(loadFactor, 1, 0.1)
	}
}
func (d *dict) elasticThreshold(loadFactor, incr, decr float64) int8 {
	switch {
	case loadFactor >= incr:
		return ElasticIncr
	case loadFactor < decr:
		return ElasticDecr
	default:
		return ElasticNone
	}
}

/* 删除一个元素，成功返回DICT_OK，元素找不到则返回 DICT_ERR */
func dictDelete(ht *dict, key unsafe.Pointer) int {
	if dictGenericDelete(ht,key,0)  {
		return DICT_OK
	}

	return DICT_ERR
}

/* Search and remove an element. This is a helper function for
 * dictDelete() and dictUnlink(), please check the top comment
 * of those functions. */
func dictGenericDelete(d *dict,  key unsafe.Pointer, nofree int) *dictEntry {
	var h, idx uint64_t
	var he, prevHe *dictEntry
	var table int

	/* dict 是空的 */
	if dictSize(d) == 0 {
		return nil
	}

	if dictIsRehashing(d) {
		_dictRehashStep(d)
	}
	h = dictHashKey(d, key)

	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		he = d.ht_table[table][idx];
		prevHe = NULL;
		while(he) {
		if (key==he.key || dictCompareKeys(d, key, he.key)) {
		/* Unlink the element from the list */
		if (prevHe)
		prevHe.next = he.next;
		else
		d.ht_table[table][idx] = he.next;
		if (!nofree) {
		dictFreeUnlinkedEntry(d, he);
	}
	d.ht_used[table]--;
	return he;
	}
	prevHe = he;
	he = he->next;
	}
	if (!dictIsRehashing(d)) break;
	}
	return NULL; /* not found */
}

func _dictRehashStep(d *dict) {
	if d.pauserehash == 0 {
		dictRehash(d,1)
	}
}

/* Performs N steps of incremental rehashing. Returns 1 if there are still
 * keys to move from the old to the new hash table, otherwise 0 is returned.
 *
 * Note that a rehashing step consists in moving a bucket (that may have more
 * than one key as we use chaining) from the old to the new hash table, however
 * since part of the hash table may be composed of empty spaces, it is not
 * guaranteed that this function will rehash even a single bucket, since it
 * will visit at max N*10 empty buckets in total, otherwise the amount of
 * work it does would be unbound and the function may block for a long time.

*/
func dictRehash(d *dict, n int) int {
	var empty_visits int = n * 10 /* Max number of empty buckets to visit. */
	if !dictIsRehashing(d) {
		return 0
	}

	n--
	for n != 0 && d.ht_used[0] != 0 {//到底是先 n!=0 再n--?//TODO
		/* 注意rehashidx不能溢出，因为ht[0].used !=0 以确保还有更多的元素 Note that rehashidx can't overflow as we are sure there are more
		 * elements because ht[0].used != 0 */
		if DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx) {

		} else {
			_, file, line, _ := runtime.Caller(0)
			_serverAssertInRedisassert("DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx)", file, line)
			abort()
		}

		for d.ht_table[0] != nil && d.ht_table[0][d.rehashidx] == nil {
			d.rehashidx++//继续遍历下一个x，d.ht_table[0][x]
			empty_visits--
			if empty_visits == 0 {//如果 进行了empty_visits次的 空遍历 则先返回
				return 1
			}
		}

		de := d.ht_table[0][d.rehashidx]
		/* 把全部在旧hashtable的bucket里的key移到新hashtable的bucket里 */
		for de != nil {
			nextde := de.next

			/* 获取在新hash table的index */
			newHashIdx := d.typei.hashFunction(de.key) & DICTHT_SIZE_MASK(d.ht_size_exp[1])
			de.next = d.ht_table[1][newHashIdx]
			d.ht_table[1][newHashIdx] = de

			d.ht_used[0]--
			d.ht_used[1]++

			de = nextde
		}

		//当前rehashidx的bucket下的全部key都搬移完毕
		d.ht_table[0][d.rehashidx] = nil
		d.rehashidx++

		n--//剩余的搬移操作次数-1
	}


	/* 如果已经rehash了整个table... */
	if d.ht_used[0] == 0 {
		zfree(unsafe.Pointer(&d.ht_table[0]))
		/* 把新ht_table的指向赋给ht_table[0] */
		d.ht_table[0] = d.ht_table[1]
		d.ht_used[0] = d.ht_used[1]
		d.ht_size_exp[0] = d.ht_size_exp[1]
		_dictReset(d, 1)
		d.rehashidx = -1
		return 0
	}

	/* 还要继续rehash操作...（渐进式rehash）*/
	return 1
}

/* ----------------------------- API implementation ------------------------- */

/* Reset hash table parameters already initialized with _dictInit()*/
func _dictReset(d *dict, htidx int) {
	d.ht_table[htidx] = nil
	d.ht_size_exp[htidx] = -1
	d.ht_used[htidx] = 0
}