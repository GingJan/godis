package src

//字典结构
import (
	"math"
	"unsafe"
)

const (
	runningCmd     = 0
	BG_SAVE        = 1
	BG_REWRITE_AOF = 2

	ElasticNone = 0
	ElasticIncr = 1
	ElasticDecr = 2
)

type dictht struct {
	table    []*dictEntry
	size     uint64
	sizemask uint64
	used     uint64
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
	dict.ht = [2]*dictht{dictht0, dictht1}
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
	if d.rehashidx > -1 { //正在进行rehash，所以ht[0]只减不增
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
	if d.rehashidx == -1 { //本轮第一次扩容
		d.rehashidx = 0
		incrToSize := uint64(math.Pow(float64(2), math.Log2(float64(d.ht[0].used)*2)+1)) //2^n次方
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
			if d.rehashidx >= len(d.ht[0].table) { //完成扩容
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
			d.ht[0].table[d.rehashidx] = curr.next //移动到下一个
			//delete(curr)//释放当前curr
			d.ht[0].used--

			return //每次操作只进行一次弹性扩容/缩小
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

/* 删除一个元素，是dictDelete()和dictUnlink()的辅助函数，请查阅这两函数的注释
本函数会先遍历查找key对应的entry，然后再删除之。
*/
func dictGenericDelete(d *dict, key unsafe.Pointer, nofree bool) *dictEntry {
	var h, idx uint64_t
	var he, prevHe *dictEntry
	var table int

	/* dict 是空的 */
	if dictSize(d) == 0 {
		return nil
	}

	if dictIsRehashing(d) {
		_dictRehashStep(d) //进行rehash操作
	}

	h = dictHashKey(d, key) //计算key的hash值
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		he = (*(d.ht_table[table]))[idx]
		prevHe = nil
		for he != nil {
			if key == he.key || dictCompareKeys(d, key, he.key) {
				/* 从list里unlink该key（对应的entry） */
				if prevHe != nil {
					prevHe.next = he.next
				} else {
					(*(d.ht_table[table]))[idx] = he.next
				}

				if !nofree { //需要释放被删元素的占用空间
					dictFreeUnlinkedEntry(d, he) //释放he占的空间
				}

				d.ht_used[table]-- //已用个数-1
				return he
			}

			prevHe = he //bucket里的拉链的第一个entry并不是要删除的key，遍历到下一个key进行
			he = he.next
		}

		if !dictIsRehashing(d) {
			break
		}
	}

	return nil /* not found */
}

/* 从ht里删除并释放 key，成功返回DICT_OK，元素找不到则返回 DICT_ERR
本函数会先查找key对应的entry，然后再删除之。
*/
func dictDelete(ht *dict, key unsafe.Pointer) int {
	if dictGenericDelete(ht, key, false) != nil { //dictGenericDelete函数会先遍历查找key对应的entry，然后再删除之。
		return DICT_OK
	}

	return DICT_ERR
}

/* 从ht里删除key，但不释放key的空间 */
func dictUnlink(d *dict, key unsafe.Pointer) *dictEntry {
	return dictGenericDelete(d, key, true)
}

/* 调用dictUnlink()后，调用本函数才真正地释放he占的空间，当he为nil时，调用该函数也是安全的 */
func dictFreeUnlinkedEntry(d *dict, he *dictEntry) {
	if he == nil {
		return
	}

	dictFreeKey(d, he)        //释放he对应的key
	dictFreeVal(d, he)        //释放he对应的val
	zfree(unsafe.Pointer(he)) //把he释放
}

/* 摧毁一整个dict字典，htidx只可传0（旧ht）和1（新ht） */
func _dictClear(d *dict, htidx int, callback func(*dict)) int {
	var i uint64

	/* 释放全部元素 */
	for i = 0; i < DICTHT_SIZE(d.ht_size_exp[htidx]) && d.ht_used[htidx] > 0; i++ {
		var he, nextHe *dictEntry

		if callback != nil && (i&65535) == 0 {
			callback(d)
		}

		he = (*(d.ht_table[htidx]))[i]
		if he == nil {
			continue
		}

		for he != nil {
			nextHe = he.next
			dictFreeKey(d, he)
			dictFreeVal(d, he)
			zfree(unsafe.Pointer(he))
			d.ht_used[htidx]--
			he = nextHe
		}
	}

	/* 释放table的空间和分配的缓存结构 */
	zfree(unsafe.Pointer(&d.ht_table[htidx]))

	_dictReset(d, htidx)

	return DICT_OK
}

/* 清除并释放hash table */
func dictRelease(d *dict) {
	_dictClear(d, 0, nil)
	_dictClear(d, 1, nil)
	zfree(unsafe.Pointer(d))
}

//寻找key对应的entry
func dictFind(d *dict, key unsafe.Pointer) *dictEntry {
	var he *dictEntry
	var h, idx, table uint64_t

	if dictSize(d) == 0 { /* 字典是空的 */
		return nil
	}

	if dictIsRehashing(d) {
		_dictRehashStep(d) //执行渐进式rehash
	}

	h = dictHashKey(d, key) //计算key的hash值
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		he = (*(d.ht_table[table]))[idx]
		for he != nil {
			if key == he.key || dictCompareKeys(d, key, he.key) { //是要找的key
				return he //返回key对应的entry
			}

			he = he.next //继续遍历拉链里的下一个entry
		}

		if !dictIsRehashing(d) {
			return nil
		}
	}

	return nil
}

//获取key对应的val
func dictFetchValue(d *dict, key unsafe.Pointer) unsafe.Pointer {
	he := dictFind(d, key)
	if he != nil {
		return dictGetVal(he)
	}
	return nil
}

/* 从ht里查找元素，若找到元素则返回对应entry，接着调用者应调用`dictTwoPhaseUnlinkFree`函数以unlink和释放该entry。
若找不到该元素则返回nil。plink存放的是对应entry的指针。
本函数和`dictTwoPhaseUnlinkFree`应一起成对调用，`dictTwoPhaseUnlinkFind`暂停rehash而`dictTwoPhaseUnlinkFree`恢复rehash

可按照以下方式使用：
var de *dictEntry = dictTwoPhaseUnlinkFind(db.dict,key.ptr,&plink, &table)
//其他代码（但不能修改dict）
dictTwoPhaseUnlinkFree(db.dict,de,plink,table); // 不需要再去找一次
这两函数配合起来，就是先找到要删元素，使用元素，然后删除元素

如果想要再删除某个entry前先查找出来，可按以上示例使用，这样避免先dictFind()再dictDelete()（因为这两函数都会有寻找entry的过程，寻找重复做了两次）
*/
func dictTwoPhaseUnlinkFind(d *dict, key unsafe.Pointer, plink ***dictEntry, table_index *int) *dictEntry {
	var h, idx, table uint64_t

	if dictSize(d) == 0 { /* dict is empty */
		return nil
	}

	//进行渐进式rehash
	if dictIsRehashing(d) {
		_dictRehashStep(d)
	}

	h = dictHashKey(d, key)
	//遍历查找key对应entry
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		var ref **dictEntry = &((*(d.ht_table[table]))[idx])
		for *ref != nil {
			if key == (*ref).key || dictCompareKeys(d, key, (*ref).key) {
				*table_index = int(table)
				*plink = ref
				dictPauseRehashing(d) //暂停rehash
				return *ref
			}
			ref = &(*ref).next
		}

		if !dictIsRehashing(d) { //如果不在进行rehash，则无需遍历ht_table[1]了
			return nil
		}
	}

	return nil
}

// 把he从d里移除并释放he的空间
func dictTwoPhaseUnlinkFree(d *dict, he *dictEntry, plink **dictEntry, table_index int) {
	if he == nil {
		return
	}

	d.ht_used[table_index]--
	*plink = he.next

	dictFreeKey(d, he)
	dictFreeVal(d, he)
	zfree(unsafe.Pointer(he))
	dictResumeRehashing(d) //恢复rehash
}

/* 返回的64位fingerprint数字表示dict字典在指定时间的状态，由几个dict的字段通过 异或 运算得出，
 * at a given time, it's just a few dict properties xored together.
 * When an unsafe iterator is initialized, we get the dict fingerprint, and check
 * the fingerprint again when the iterator is released.
 * If the two fingerprints are different it means that the user of the iterator
 * performed forbidden operations against the dictionary while iterating. */
func dictFingerprint(d *dict) uint64 {
	var integers [6]uint64
	var j int

	integers[0] = uint64(uintptr(unsafe.Pointer(d.ht_table[0])))
	integers[1] = uint64(d.ht_size_exp[0])
	integers[2] = d.ht_used[0]
	integers[3] = uint64(uintptr(unsafe.Pointer(d.ht_table[1])))
	integers[4] = uint64(d.ht_size_exp[1])
	integers[5] = d.ht_used[1]

	/* 使用类似以下公式计算最后的hash值:
	 * Result = hash(hash(hash(int1)+int2)+int3) ...
	 * 几个hash的顺序不同，得出的最终hash值也不同
	 */
	var hash uint64 = 0
	for j = 0; j < 6; j++ {
		hash += integers[j]
		/* For the hashing step we use Tomas Wang's 64 bit integer hash. */
		hash = (^hash) + (hash << 21) // hash = (hash << 21) - hash - 1;
		hash = hash ^ (hash >> 24)
		hash = (hash + (hash << 3)) + (hash << 8) // hash * 265
		hash = hash ^ (hash >> 14)
		hash = (hash + (hash << 2)) + (hash << 4) // hash * 21
		hash = hash ^ (hash >> 28)
		hash = hash + (hash << 31)
	}
	return hash
}

//初始化iterator
func dictInitIterator(iter *dictIterator, d *dict) {
	iter.d = d
	iter.table = 0
	iter.index = -1
	iter.safe = false
	iter.entry = nil
	iter.nextEntry = nil
}

func dictInitSafeIterator(iter *dictIterator, d *dict) {
	dictInitIterator(iter, d) //初始化
	iter.safe = true
}

func dictResetIterator(iter *dictIterator) {
	if !(iter.index == -1 && iter.table == 0) {
		if iter.safe {
			dictResumeRehashing(iter.d)
		} else {
			assert(iter.fingerprint == dictFingerprint(iter.d), "iter.fingerprint == dictFingerprint(iter.d)")
		}
	}
}

func dictGetIterator(d *dict) *dictIterator {
	iter := dictIterator{}
	dictInitIterator(&iter, d)
	return &iter
}

func dictGetSafeIterator(d *dict) *dictIterator {
	i := dictGetIterator(d)
	i.safe = true
	return i
}

func dictNext(iter *dictIterator) *dictEntry {
	for {
		if iter.entry == nil {//iterator第一次迭代
			if iter.index == -1 && iter.table == 0 {//第一次迭代
				if iter.safe {//安全模式的iterator
					dictPauseRehashing(iter.d)//暂停字典的rehash
				} else {
					iter.fingerprint = dictFingerprint(iter.d)//获取当前字典的指纹
				}
			}

			iter.index++
			if iter.index >= int64(DICTHT_SIZE(iter.d.ht_size_exp[iter.table])) {//如果在迭代时，字典的大小发生变化，说明可能正在rehash。否则就是遍历完毕
				if dictIsRehashing(iter.d) && iter.table == 0 {//字典正在rehash
					iter.table++//则去迭代新的table
					iter.index = 0//从新table的0开始迭代
				} else {
					break//字典遍历完毕
				}
			}

			iter.entry = (*(iter.d.ht_table[iter.table]))[iter.index]//新table的entry
		} else {
			iter.entry = iter.nextEntry//指向到原table的下一个entry
		}

		if iter.entry != nil {
			/* 保存entry指向的下一个entry，因为返回的entry可能会被调用者删除 */
			iter.nextEntry = iter.entry.next
			return iter.entry
		}
	}

	return nil
}

void dictReleaseIterator(dictIterator *iter)
{
dictResetIterator(iter);
zfree(iter);
}

/* Return a random entry from the hash table. Useful to
 * implement randomized algorithms */
dictEntry *dictGetRandomKey(dict *d)
{
dictEntry *he, *orighe;
unsigned long h;
int listlen, listele;

if (dictSize(d) == 0) return NULL;
if (dictIsRehashing(d)) _dictRehashStep(d);
if (dictIsRehashing(d)) {
unsigned long s0 = DICTHT_SIZE(d->ht_size_exp[0]);
do {
/* We are sure there are no elements in indexes from 0
 * to rehashidx-1 */
h = d->rehashidx + (randomULong() % (dictSlots(d) - d->rehashidx));
he = (h >= s0) ? d->ht_table[1][h - s0] : d->ht_table[0][h];
} while(he == NULL);
} else {
unsigned long m = DICTHT_SIZE_MASK(d->ht_size_exp[0]);
do {
h = randomULong() & m;
he = d->ht_table[0][h];
} while(he == NULL);
}

/* Now we found a non empty bucket, but it is a linked
 * list and we need to get a random element from the list.
 * The only sane way to do so is counting the elements and
 * select a random index. */
listlen = 0;
orighe = he;
while(he) {
he = he->next;
listlen++;
}
listele = random() % listlen;
he = orighe;
while(listele--) he = he->next;
return he;
}

//执行渐进式rehash
func _dictRehashStep(d *dict) {
	if d.pauserehash == 0 {
		dictRehash(d, 1)
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

	for n != 0 && d.ht_used[0] != 0 {
		n-- //剩余的搬移操作次数-1

		/* 注意rehashidx不能溢出，因为ht[0].used !=0 以确保还有更多的元素 Note that rehashidx can't overflow as we are sure there are more
		 * elements because ht[0].used != 0 */
		assert(DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx), "DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx)")

		//找到d.rehashidx对应不为空的bucket
		for d.ht_table[0][d.rehashidx] == nil { //todo 需要做d.ht_table[0] != nil 吗
			d.rehashidx++ //继续遍历下一个x，d.ht_table[0][x]
			empty_visits--
			if empty_visits == 0 { //如果 进行了empty_visits次的 空遍历 则先返回
				return 1
			}
		}

		de := d.ht_table[0][d.rehashidx]
		/* 把全部在旧hashtable的bucket里的key移到新hashtable的bucket里 */
		for de != nil {
			nextde := de.next

			/* 获取key在新hash table的index */
			newHashIdx := d.typei.hashFunction(de.key) & DICTHT_SIZE_MASK(d.ht_size_exp[1])
			de.next = d.ht_table[1][newHashIdx]
			d.ht_table[1][newHashIdx] = de

			d.ht_used[0]-- //key被迁走，对应used-1
			d.ht_used[1]++ //key被迁入，对应used+1

			de = nextde
		}

		//当前rehashidx的bucket下的全部key都搬移完毕
		d.ht_table[0][d.rehashidx] = nil
		//继续下一个rehashidx对应的key的迁移
		d.rehashidx++
	}

	/* 如果已经rehash了整个旧table... */
	if d.ht_used[0] == 0 {
		zfree(unsafe.Pointer(&d.ht_table[0])) //释放旧table的空间
		/* 把新ht_table的指向赋给ht_table[0] */
		d.ht_table[0] = d.ht_table[1]
		d.ht_used[0] = d.ht_used[1]
		d.ht_size_exp[0] = d.ht_size_exp[1]
		//重置ht_table[1]
		_dictReset(d, 1)

		//标记rehash完毕
		d.rehashidx = -1

		return 0 //标识rehash完毕
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
