package src

import (
	"math/rand"
	"unsafe"
)

const (
	DICT_OK  = 0
	DICT_ERR = 1
)

type dictEntry struct {
	key unsafe.Pointer
	v   struct {
		val unsafe.Pointer//interface{}? todo
		u64 uint64_t
		s64 int64_t
		d   float64
	}
	next     *dictEntry        /* Next entry in the same hash bucket. */
	metadata *[]unsafe.Pointer /* An arbitrary number of bytes (starting at a
	 * pointer-aligned address) of size as returned
	 * by dictType's dictEntryMetadataBytes(). */
}

type dictType struct {
	hashFunction  func(key unsafe.Pointer) uint64_t //redis用的是MurMurHash2
	keyDup        func(d *dict, key unsafe.Pointer) unsafe.Pointer
	valDup        func(d *dict, obj unsafe.Pointer) unsafe.Pointer
	keyCompare    func(d *dict, key1 unsafe.Pointer, keys unsafe.Pointer) int
	keyDestructor func(d *dict, key unsafe.Pointer)
	valDestructor func(d *dict, obj unsafe.Pointer)
	expandAllowed func(moreMem size_t, usedRatio float64) int
	/* 允许dictEntry有由调用者定义的metadata，当dictEntry创建时，额外的metadata内存空间被初始化为0 */
	dictEntryMetadataBytes func(d *dict) size_t
}

func DICTHT_SIZE(exp int8) uint64 {//返回字典的实际大小，由exp计算得出
	if exp == -1 {
		return 0
	}

	return 1 << exp
}

func DICTHT_SIZE_MASK(exp int8) uint64 {
	if exp == -1 {
		return 0
	}

	return DICTHT_SIZE(exp) - 1
}

type dict struct {
	typei *dictType

	ht_table [2]*[]*dictEntry//TODO ht_table[0] = [(1<<exp)]*dictEntry，用切片代替未定大小的数组
	ht_used  [2]uint64 //[]*dictEntry已有*dictEntry个数

	rehashidx int64 /* rehashidx == -1 表示当前没进行rehash */

	/* Keep small vars at end for optimal (minimal) struct padding */
	pauserehash int16_t /* 如果>0，则rehash暂停了，（<0代表程序写错了） */
	ht_size_exp [2]int8 /* 空间大小的指数值，比如3代表有2^3的大小 (size = 1<<exp) */
}

/* 如果safe字段置为1，表示该iterator是安全的，也即当字典dict在遍历迭代时，对该字典调用dictAdd，dictFind和其他函数都是安全的。
否则就表示该iterator是不安全的，这样在遍历迭代时只能调用dictNext()
*/
type dictIterator struct {
	d *dict
	index int64
	table int //取值0 或 1，表示当前在遍历旧table 还是 新table
	safe bool //是否开启安全模式
	entry, nextEntry *dictEntry//迭代器当前指向的entry和下一个entry

	fingerprint uint64//当前字典d的指纹，当iter是非安全模式时，该字段用于误用检测
}

type dictScanFunction func(private unsafe.Pointer, de *dictEntry)
type dictScanBucketFunction func(d *dict, bucketref **dictEntry)

/* hash table的初始化大小 */
const DICT_HT_INITIAL_EXP = 2
const DICT_HT_INITIAL_SIZE = 1<<(DICT_HT_INITIAL_EXP)



/***** C里的宏定义 ********/

func dictFreeVal(d *dict, entry *dictEntry) {
	if d.typei.valDestructor != nil {
		d.typei.valDestructor(d, entry.v.val)
	}
}

func dictSetVal(d *dict, entry *dictEntry, _val_ unsafe.Pointer) {
	if d.typei.valDup != nil {
		entry.v.val = d.typei.valDup(d, _val_)
	} else {
		entry.v.val = _val_
	}
}


func dictSize(d *dict) uint64 {
	return (*d).ht_used[0] + (*d).ht_used[1]
}

func dictIsRehashing(d *dict) bool {
	return d.rehashidx != -1
}
//返回key的hash值
func dictHashKey(d *dict, key unsafe.Pointer) uint64_t {
	return d.typei.hashFunction(key)
}

func dictCompareKeys(d *dict, key1, key2 unsafe.Pointer) bool {
	//TODO 是否需要深度相等判断？
	if d.typei.keyCompare != nil {
		return d.typei.keyCompare(d, key1, key2) != 0
	}
	return key1 == key2
}

func dictFreeKey(d *dict, entry *dictEntry) {
	if d.typei.keyDestructor != nil {
		d.typei.keyDestructor(d, entry.key)
	}
}


func dictGetVal(he *dictEntry) unsafe.Pointer {
	return he.v.val
}

func randomULong() uint64 {
	if ULONG_MAX >= 0xffffffffffffffff {
		return uint64(rand.Int63())
	}

	return uint64(random())
}

func dictSlots(d *dict) uint64 {//新旧两个table的bucket个数总和
	return DICTHT_SIZE(d.ht_size_exp[0]) + DICTHT_SIZE(d.ht_size_exp[1])
}

func dictPauseRehashing(d *dict) {
	d.pauserehash++
}

func dictResumeRehashing(d *dict) {
	d.pauserehash--
})

/***** C里的宏指令 ********/
