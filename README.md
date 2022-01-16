# Golang-Diary

This repository is for recording my golang learning. It's very important !!! [Reference](https://draveness.me/golang/docs/). PS: The book was bought, and it's fantastic, the Diary is just the part of this

> 写了一段时间的 go, 原理\设计等方面的缺失必须要引起重视! 计划为在 2 个月内完成所有部分的阅读(源码级)

Start from: 2021-12-29 23:35

## 第一部分：预备知识

## 第二部分: 基础知识

### 第三章: 数据结构

#### 3.1 数组

##### 3.1.1 概述

通常从元素类型和元素个数来定义一个数组。跟文章中略微有些更新，在 `cmd/compile/internal/types.NewArray` 如下：

```go
// NewArray returns a new fixed-length array Type.
func NewArray(elem *Type, bound int64) *Type {
    if bound < 0 {
        base.Fatalf("NewArray: invalid bound %v", bound)
    }
    t := newType(TARRAY)
    t.extra = &Array{Elem: elem, Bound: bound}
    // chriskali: 设置类型是否分配在 GC 堆栈上
    // 官方文档: https://github.com/golang/go/blob/master/src/runtime/HACKING.md
    // 通常有全局变量，或者由 sysAlloc\persistentalloc\fixalloc分配的 或者其他手动管理的
    SetNotInHeap(elem.NotInHeap())
    // chriskali: 泛型相关
    // there is a typeparam somewhere in the type (generic function or type)
    if elem.HasTParam() {
        t.SetHasTParam(true)
    }
    // chriskali: 也是泛型相关
    // 引入的 commit: https://github.com/golang/go/commit/a7a17f0ca86d252dc1ef20b5852c352ade5f8610#diff-32ea261f6c80677968cc1fbf0a8edd91a86c3d2d9f5313cb8ac3cc901654cdf1
    // 是否能够隐式转换
    // typeIsShape: represents a set of closely related types, for generics
    if elem.HasShape() {
        t.SetHasShape(true)
    }
    return t
}
```

能看到相比之前的，go 1.18 beta 开始着手于泛型相关。看来 2022 年可以见到带有泛型的 go 1.18 了

##### 3.1.2 初始化

初始化的方式有两种，一种固定长度，一种推导。推导在编译时候就会被转换成为固定长度

```go
arr1 := [3]int{1, 2, 3}
arr2 := [...]int{1, 2, 3}
```

###### 上限推导

这部分的源码和书中的略有不同，在 `cmd/compile/internal/typecheck/expr.go#tcCompLit` 下

```golang
// 	n.Left = tcCompLit(n.Left)
func tcCompLit(n *ir.CompLitExpr) (res ir.Node) {
    ...
	// Need to handle [...]T arrays specially.
	if array, ok := n.Ntype.(*ir.ArrayType); ok && array.Elem != nil && array.Len == nil {
		array.Elem = typecheckNtype(array.Elem)
		elemType := array.Elem.Type()
		if elemType == nil {
			n.SetType(nil)
			return n
		}
		length := typecheckarraylit(elemType, -1, n.List, "array literal")
		n.SetOp(ir.OARRAYLIT)
		n.SetType(types.NewArray(elemType, length))
		n.Ntype = nil
		return n
	}
    ...
	switch t.Kind() {
    ...
	case types.TARRAY:
		typecheckarraylit(t.Elem(), t.NumElem(), n.List, "array literal")
		n.SetOp(ir.OARRAYLIT)
		n.Ntype = nil
    ...
	}

	return n
}
```

可以看到关键函数为 `typecheckarraylit` 中对 length 的推导，继续跟进，源码如下：

```golang
// typecheckarraylit type-checks a sequence of slice/array literal elements.
func typecheckarraylit(elemType *types.Type, bound int64, elts []ir.Node, ctx string) int64 {
	// If there are key/value pairs, create a map to keep seen
	// keys so we can check for duplicate indices.
	var indices map[int64]bool
	for _, elt := range elts {
		if elt.Op() == ir.OKEY {
			indices = make(map[int64]bool)
			break
		}
	}

	var key, length int64
	for i, elt := range elts {
		ir.SetPos(elt)
		r := elts[i]
		var kv *ir.KeyExpr
        // 如果 elt 是 key/value 类型
        // ir.OKEY => Key:Value (key:value in struct/array/map literal)
		if elt.Op() == ir.OKEY {
			elt := elt.(*ir.KeyExpr)
			elt.Key = Expr(elt.Key)
			key = IndexConst(elt.Key)
			if key < 0 {
				if !elt.Key.Diag() {
					if key == -2 {
						base.Errorf("index too large")
					} else {
						base.Errorf("index must be non-negative integer constant")
					}
					elt.Key.SetDiag(true)
				}
				key = -(1 << 30) // stay negative for a while
			}
			kv = elt
			r = elt.Value
		}

		r = pushtype(r, elemType)
		r = Expr(r)
		r = AssignConv(r, elemType, ctx)
		if kv != nil {
			kv.Value = r
		} else {
			elts[i] = r
		}

		if key >= 0 {
			if indices != nil {
				if indices[key] {
					base.Errorf("duplicate index in %s: %d", ctx, key)
				} else {
					indices[key] = true
				}
			}

			if bound >= 0 && key >= bound {
				base.Errorf("array index %d out of bounds [0:%d]", key, bound)
				bound = -1
			}
		}

		key++
		if key > length {
			length = key
		}
	}

	return length
}
```

这一部分涉及到 golang 相关的编译处理(暂时还不会)，`CompLitExpr` 等 `ir` 相关的在 `cmd/compile/internal/ir` 下。通过遍历传递进来的 `ir.Node` 来遍历元素(中间对 key/value 形式做了别的判断, 暂时还没看) 来进行数量上的计算，最后返回 length

可见两种写法在运行时是一样的，在编译阶段通过对 `[...]T{...}` 形式的推导，来完成 length 的计算

###### 语句转换

> 先只看 ARRAY 相关的部分

```golang
func anylit(n ir.Node, var_ ir.Node, init *ir.Nodes) {
    ...
	case ir.OSTRUCTLIT, ir.OARRAYLIT:
		n := n.(*ir.CompLitExpr)
		if !t.IsStruct() && !t.IsArray() {
			base.Fatalf("anylit: not struct/array")
		}
        // chriskali
        // 关键点，判断长度是否大于 4 
		if isSimpleName(var_) && len(n.List) > 4 {
			// lay out static data
			vstat := readonlystaticname(t)

			ctxt := inInitFunction
			if n.Op() == ir.OARRAYLIT {
				ctxt = inNonInitFunction
			}
			fixedlit(ctxt, initKindStatic, n, vstat, init)

			// copy static to var
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, vstat))

			// add expressions to automatic
			fixedlit(inInitFunction, initKindDynamic, n, var_, init)
			break
		}

		var components int64
		if n.Op() == ir.OARRAYLIT {
			components = t.NumElem()
		} else {
			components = int64(t.NumFields())
		}
		// initialization of an array or struct with unspecified components (missing fields or arrays)
		if isSimpleName(var_) || int64(len(n.List)) < components {
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, nil))
		}

		fixedlit(inInitFunction, initKindLocalCode, n, var_, init)
    ...
	}
}
```

可以看到判断的临界点为元素个数是否大于4。跟进 `fixedlit` 函数，主要看对 `Kind` 的处理

```golang
func fixedlit(ctxt initContext, kind initKind, n *ir.CompLitExpr, var_ ir.Node, init *ir.Nodes) {
	isBlank := var_ == ir.BlankNode
	var splitnode func(ir.Node) (a ir.Node, value ir.Node)
	...
	for _, r := range n.List {
		a, value := splitnode(r)
		...
		// build list of assignments: var[index] = expr
		ir.SetPos(a)
		as := ir.NewAssignStmt(base.Pos, a, value)
		as = typecheck.Stmt(as).(*ir.AssignStmt)
		switch kind {
		case initKindStatic:
			genAsStatic(as)
		// 当 len 小于等于 4 时，Kind 为 initKindLocalCode
		case initKindDynamic, initKindLocalCode:
			a = orderStmtInPlace(as, map[string][]*ir.Name{})
			a = walkStmt(a)
			init.Append(a)
		default:
			base.Fatalf("fixedlit: bad kind %d", kind)
		}
	}
}
```

当 `len` 小于等于 4 的时候，关键函数为 `orderStmtInPlace`，我们看一下对他的定义

```golang

// orderStmtInPlace orders the side effects of the single statement *np
// and replaces it with the resulting statement list.
// The result of orderStmtInPlace MUST be assigned back to n, e.g.
// 	n.Left = orderStmtInPlace(n.Left)
// free is a map that can be used to obtain temporary variables by type.
func orderStmtInPlace(n ir.Node, free map[string][]*ir.Name) ir.Node {
	var order orderState
	order.free = free
	mark := order.markTemp()
	order.stmt(n)
	order.cleanTemp(mark)
	return ir.NewBlockStmt(src.NoXPos, order.out)
}
```

看 desc 的大致含义为，将字段进行排序并替换为 `resulting statement list` ，看了作者的解释，大致为：

```golang
var arr [3]int
arr[0] = 1
arr[1] = 2
arr[2] = 3
```

当 `len` 大于 4 的时候，会先调用 `readonlystaticname` 获取一个 `staticname`，在静态存储区(`kind` 为 `initKindStatic`) 初始化，然后将 `staticname` append 到这个数组中，最后和第一种情况一样，进行展开，copy 一下作者的伪代码

```golang
var arr [5]int
statictmp_0[0] = 1
statictmp_0[1] = 2
statictmp_0[2] = 3
statictmp_0[3] = 4
statictmp_0[4] = 5
arr = statictmp_0
```

#### 3.1.3 访问和赋值

checks 代码函数为 `tcIndex`

```golang
func tcIndex(n *ir.IndexExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	n.X = implicitstar(n.X)
	l := n.X
	n.Index = Expr(n.Index)
	r := n.Index
	t := l.Type()
	if t == nil || r.Type() == nil {
		n.SetType(nil)
		return n
	}
	switch t.Kind() {
	...
	case types.TSTRING, types.TARRAY, types.TSLICE:
		n.Index = indexlit(n.Index)
		if t.IsString() {
			n.SetType(types.ByteType)
		} else {
			n.SetType(t.Elem())
		}
		why := "string"
		if t.IsArray() {
			why = "array"
		} else if t.IsSlice() {
			why = "slice"
		}

		// 必须为正整数
		if n.Index.Type() != nil && !n.Index.Type().IsInteger() {
			base.Errorf("non-integer %s index %v", why, n.Index)
			return n
		}

		if !n.Bounded() && ir.IsConst(n.Index, constant.Int) {
			x := n.Index.Val()
			// 判断是否 negative
			if constant.Sign(x) < 0 {
				base.Errorf("invalid %s index %v (index must be non-negative)", why, n.Index)
			// 越界 check
			} else if t.IsArray() && constant.Compare(x, token.GEQ, constant.MakeInt64(t.NumElem())) {
				base.Errorf("invalid array index %v (out of bounds for %d-element array)", n.Index, t.NumElem())
			// same
			} else if ir.IsConst(n.X, constant.String) && constant.Compare(x, token.GEQ, constant.MakeInt64(int64(len(ir.StringVal(n.X))))) {
				base.Errorf("invalid string index %v (out of bounds for %d-byte string)", n.Index, len(ir.StringVal(n.X)))
			// overflow 检查
			} else if ir.ConstOverflow(x, types.Types[types.TINT]) {
				base.Errorf("invalid %s index %v (index too large)", why, n.Index)
			}
		}
		...
	return n
}
```

以上是在编译期间的对数组的 check。在生成中间代码期间，还会插入运行时方法`runtime.panicIndex` 调用防止发生越界错误。这里我跳过了 ssa 的部分，先不看到那么深...

#### 3.2 切片

> 相比数组，切片更为常用

创建部分的代码和书中的代码基本一样，只是和数组一样多了泛型相关判断

```golang
func NewSlice(elem *Type) *Type {
	if t := elem.cache.slice; t != nil {
		if t.Elem() != elem {
			base.Fatalf("elem mismatch")
		}
		if elem.HasTParam() != t.HasTParam() || elem.HasShape() != t.HasShape() {
			base.Fatalf("Incorrect HasTParam/HasShape flag for cached slice type")
		}
		return t
	}

	t := newType(TSLICE)
	// extra 字段来附加类型，帮助运行时的动态获取
	t.extra = Slice{Elem: elem}
	elem.cache.slice = t
	if elem.HasTParam() {
		t.SetHasTParam(true)
	}
	if elem.HasShape() {
		t.SetHasShape(true)
	}
	return t
}
```

##### 3.2.1 数据结构

```golang
// SliceHeader is the runtime representation of a slice.
// It cannot be used safely or portably and its representation may
// change in a later release.
// Moreover, the Data field is not sufficient to guarantee the data
// it references will not be garbage collected, so programs must keep
// a separate, correctly typed pointer to the underlying data.
type SliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}
```

熟悉的 `Len` 和 `Cap`。`Data` 即指向一片连续的内存，这和后面 runtime 中数组的操作有关

##### 3.2.2 初始化

三种方式

```golang
arr[0:3] or slice[0:3]
slice := []int{1, 2, 3}
slice := make([]int, 10)
```

**使用下标**

在作者展示的 SSA 代码中，能看到接收了几个参数，初始化一个 `array`，将 ptr 指向到 array，然后赋值 `cap` 和 `len`。

**字面量**

在编译期间，展开为

```golang
var vstat [3]int
vstat[0] = 1
vstat[1] = 2
vstat[2] = 3
var vauto *[3]int = new([3]int)
*vauto = vstat
slice := vauto[:]
```

**关键字**

运行时参与，关键函数 `typecheck1`

```golang
func typecheck1(n ir.Node, top int) ir.Node {
	...
	switch n.Op() {
	...
	case ir.OMAKE:
	n := n.(*ir.CallExpr)
	return tcMake(n)
	...
	}
}
```

跟进

```golang
func tcMake(n *ir.CallExpr) ir.Node {
	args := n.Args
	if len(args) == 0 {
		base.Errorf("missing argument to make")
		n.SetType(nil)
		return n
	}
	// 取第一个参数
	n.Args = nil
	l := args[0]
	l = typecheck(l, ctxType)
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	i := 1
	var nn ir.Node
	switch t.Kind() {
		...
		case types.TSLICE:
		// 检查是否传递 len
		if i >= len(args) {
			base.Errorf("missing len argument to make(%v)", t)
			n.SetType(nil)
			return n
		}

		l = args[i]
		i++
		l = Expr(l)
		var r ir.Node
		if i < len(args) {
			r = args[i]
			i++
			r = Expr(r)
		}
		// 类型判断
		if l.Type() == nil || (r != nil && r.Type() == nil) {
			n.SetType(nil)
			return n
		}
		if !checkmake(t, "len", &l) || r != nil && !checkmake(t, "cap", &r) {
			n.SetType(nil)
			return n
		}
		// cap 必须 >= len
		if ir.IsConst(l, constant.Int) && r != nil && ir.IsConst(r, constant.Int) && constant.Compare(l.Val(), token.GTR, r.Val()) {
			base.Errorf("len larger than cap in make(%v)", t)
			n.SetType(nil)
			return n
		}
		nn = ir.NewMakeExpr(n.Pos(), ir.OMAKESLICE, l, r)
	}
}
```

判断以及校验，创建切片的 `runtime` 函数为

```golang
func makeslice(et *_type, len, cap int) unsafe.Pointer {
	mem, overflow := math.MulUintptr(et.size, uintptr(cap))
	if overflow || mem > maxAlloc || len < 0 || len > cap {
		// NOTE: Produce a 'len out of range' error instead of a
		// 'cap out of range' error when someone does make([]T, bignumber).
		// 'cap out of range' is true too, but since the cap is only being
		// supplied implicitly, saying len is clearer.
		// See golang.org/issue/4085.
		mem, overflow := math.MulUintptr(et.size, uintptr(len))
		if overflow || mem > maxAlloc || len < 0 {
			panicmakeslicelen()
		}
		panicmakeslicecap()
	}

	return mallocgc(mem, et, true)
}
```

##### 3.2.3 访问元素

##### 3.2.4 追加扩容

两种情况处理，是否需要将 slice 赋值给原来的 slice。对于赋给原有变量的做了优化，不用担心拷贝发生的性能影响。

**追加元素**

这一部分代码和之前的不一样

```golang
func growslice(et *_type, old slice, cap int) slice {
	...
	newcap := old.cap
	doublecap := newcap + newcap
	// chriskali
	// 点1：如果大于当前容量的两倍，则直接扩容到期望值
	if cap > doublecap {
		newcap = cap
	// 当在2倍以内时
	} else {
		// 和文章中不一样，threshold 变成 256 了
		const threshold = 256
		// 点2：当小于 256 的时候，翻倍
		if old.cap < threshold {
			newcap = doublecap
		} else {
			// Check 0 < newcap to detect overflow
			// and prevent an infinite loop.
			// 点3：循环 25% 的增加，直到大于期望的值
			for 0 < newcap && newcap < cap {
				// Transition from growing 2x for small slices
				// to growing 1.25x for large slices. This formula
				// gives a smooth-ish transition between the two.
				newcap += (newcap + 3*threshold) / 4
			}
			// Set newcap to the requested cap when
			// the newcap calculation overflowed.
			if newcap <= 0 {
				newcap = cap
			}
		}
	}

	var overflow bool
	var lenmem, newlenmem, capmem uintptr
	// Specialize for common values of et.size.
	// For 1 we don't need any division/multiplication.
	// For sys.PtrSize, compiler will optimize division/multiplication into a shift by a constant.
	// For powers of 2, use a variable shift.
	// 点4：1、8、2的倍数做内存对齐。roundupsize 函数
	switch {
	case et.size == 1:
		lenmem = uintptr(old.len)
		newlenmem = uintptr(cap)
		capmem = roundupsize(uintptr(newcap))
		overflow = uintptr(newcap) > maxAlloc
		newcap = int(capmem)
	case et.size == goarch.PtrSize:
		lenmem = uintptr(old.len) * goarch.PtrSize
		newlenmem = uintptr(cap) * goarch.PtrSize
		capmem = roundupsize(uintptr(newcap) * goarch.PtrSize)
		overflow = uintptr(newcap) > maxAlloc/goarch.PtrSize
		newcap = int(capmem / goarch.PtrSize)
	case isPowerOfTwo(et.size):
		var shift uintptr
		if goarch.PtrSize == 8 {
			// Mask shift for better code generation.
			shift = uintptr(sys.Ctz64(uint64(et.size))) & 63
		} else {
			shift = uintptr(sys.Ctz32(uint32(et.size))) & 31
		}
		lenmem = uintptr(old.len) << shift
		newlenmem = uintptr(cap) << shift
		capmem = roundupsize(uintptr(newcap) << shift)
		overflow = uintptr(newcap) > (maxAlloc >> shift)
		newcap = int(capmem >> shift)
	default:
		...
	}
	...
}
```

##### 3.2.5 拷贝切片

简单来说 `runtime.memmove` 整个拷贝，新建 `SliceHeader` 将 `Data` ptr 指向到新建的内存。整段拷贝依然会消耗比较大的资源

#### 3.3 哈希表

> 核心思想：若关键字为 k，则其值存放在 f(k) 的存储位置上。由此，不需比较便可直接取得所查记录。在 `golang` 中，重点关注 `runtime/map.go` 下的实现

##### 3.3.1 设计原理

> 非常重要的数据结构之一。关键词：数据结构，哈希函数，冲突解决方法。建立一个合理的均匀分布 key 以及 冲突的处理 十分关键

**哈希函数**

> 补充一下

- 直接定址法
- 数字分析法
- 平方取中法
- 折叠法
- 随机数法
- 除留余数法

**冲突解决**

将无限映射到有限，一定会有冲突的问题。目前提到的冲突并不是哈希完全相等，而是部分，例如前几个字节相同

常见的处理冲突的方式有：1. 开放寻址法 2. 拉链法 (百度的一个 3. 桶定址法)

- 开放寻址法([Reference](https://en.wikipedia.org/wiki/Open_addressing))

	核心思想为：依次探测和比较数组中的元素以判断目标键值对是否存在于哈希表中。当写入数据的时候，如果发生了冲突，就会将键值对写入到下一个索引不为空的位置
	这一块跟语言无关，所以直接看[哈希表-Reference-1](https://zh.wikipedia.org/wiki/%E5%93%88%E5%B8%8C%E8%A1%A8)，更加直接一点。 增量的

	- Linear Probing: 逐个弹出额存放地址的表，直到查找到一个空单元，把散列地址存放在该空单元
	- Quadratic Probing: 平方探测
	- Double hashing: 用另外一个 hash function 来做二次随机

- 拉链法

	拉链法的实现一般为数组 + 链表的形式。由于其平均查找时间短，存储节点的内存都是动态申请，节省内存空间。也是实现的最常见的方式。这个会在别的仓库里重新过一遍

	有一个比较重要的概念 - 装载因子，装载因子越大，填入的数据越多，空间利用率就越高，但是发生 hash 冲突的的概率越大。在拉链法中，装载因子为

	装载因子 := 元素数量 / 桶数量

	在 golang 中装载因子固定的为 6.5，即每个 bucket 平均存储的 kv 超过 6.5 个的时候，就会进行扩容

##### 3.3.2 数据结构

```golang
// A header for a Go map.
type hmap struct {
	count     int // 元素个数
	flags     uint8	// 状态位
	B         uint8  // log_2 of # of buckets (can hold up to loadFactor * 2^B items)，通过 B 值来计算
	noverflow uint16 // 溢出桶的大致数量
	hash0     uint32 // hash seed

	buckets    unsafe.Pointer // array of 2^B Buckets. may be nil if count==0.
	oldbuckets unsafe.Pointer // 发生扩容，old buckets 指向老 buckets，长度为新的 1/2
	nevacuate  uintptr        // progress counter for evacuation (buckets less than this have been evacuated)

	extra *mapextra // 优化 GC 扫描而设定的
}

type mapextra struct {
	// If both key and elem do not contain pointers and are inline, then we mark bucket
	// type as containing no pointers. This avoids scanning such maps.
	// However, bmap.overflow is a pointer. In order to keep overflow buckets
	// alive, we store pointers to all overflow buckets in hmap.extra.overflow and hmap.extra.oldoverflow.
	// overflow and oldoverflow are only used if key and elem do not contain pointers.
	// overflow contains overflow buckets for hmap.buckets.
	// oldoverflow contains overflow buckets for hmap.oldbuckets.
	// The indirection allows to store a pointer to the slice in hiter.
	overflow    *[]*bmap
	oldoverflow *[]*bmap

	// nextOverflow holds a pointer to a free overflow bucket.
	nextOverflow *bmap
}

// A bucket for a Go map.
type bmap struct {
	// tophash generally contains the top byte of the hash value
	// for each key in this bucket. If tophash[0] < minTopHash,
	// tophash[0] is a bucket evacuation state instead.
	tophash [bucketCnt]uint8
	// Followed by bucketCnt keys and then bucketCnt elems.
	// NOTE: packing all the keys together and then all the elems together makes the
	// code a bit more complicated than alternating key/elem/key/elem/... but it allows
	// us to eliminate padding which would be needed for, e.g., map[int64]int8.
	// Followed by an overflow pointer.
}
```

作者博客里的图非常好，帮助我们理解

![map-1](https://img.draveness.me/2020-10-18-16030322432679/hmap-and-buckets.png)

另外附上一个[博客](https://phati-sawant.medium.com/internals-of-map-in-golang-33db6e25b3f8)里的图片

![map-2](https://miro.medium.com/max/700/1*WIK6OKROozuefipgikW-8Q.png)

首先 `hmap` 指向一个 `bucket array` ，每个 `bucket` (即 `bmap`) 存储至多 8 个键值对。在 `hmap` 中的 `extra` 字段存储为溢出桶

下面是关键的常量信息

```golang
const (
	// Maximum number of key/elem pairs a bucket can hold.
	bucketCntBits = 3
	bucketCnt     = 1 << bucketCntBits

	// 装载因子为 6.5
	loadFactorNum = 13
	loadFactorDen = 2

	// Maximum key or elem size to keep inline (instead of mallocing per element).
	// Must fit in a uint8.
	// Fast versions cannot handle big elems - the cutoff size for
	// fast versions in cmd/compile/internal/gc/walk.go must be at most this elem.
	maxKeySize  = 128
	maxElemSize = 128

	// data offset should be the size of the bmap struct, but needs to be
	// aligned correctly. For amd64p32 this means 64-bit alignment
	// even though pointers are 32 bit.
	dataOffset = unsafe.Offsetof(struct {
		b bmap
		v int64
	}{}.v)

	emptyRest      = 0 // this cell is empty, and there are no more non-empty cells at higher indexes or overflows.
	emptyOne       = 1 // this cell is empty
	evacuatedX     = 2 // key/elem is valid.  Entry has been evacuated to first half of larger table.
	evacuatedY     = 3 // same as above, but evacuated to second half of larger table.
	evacuatedEmpty = 4 // cell is empty, bucket is evacuated.
	minTopHash     = 5 // minimum tophash for a normal filled cell.

	// flags
	iterator     = 1 // there may be an iterator using buckets
	oldIterator  = 2 // there may be an iterator using oldbuckets
	hashWriting  = 4 // a goroutine is writing to the map
	sameSizeGrow = 8 // the current map growth is to a new map of the same size

	// sentinel bucket ID for iterator checks
	noCheck = 1<<(8*goarch.PtrSize) - 1
)
```

![map-3](https://segmentfault.com/img/bVcIsJO)

为什么是 6.5 呢，在 runtime.map 中找到一个注释，他们做了一下解释和 benchmark

```golang
// Picking loadFactor: too large and we have lots of overflow
// buckets, too small and we waste a lot of space. I wrote
// a simple program to check some stats for different loads:
// (64-bit, 8 byte keys and elems)
//  loadFactor    %overflow  bytes/entry     hitprobe    missprobe
//        4.00         2.13        20.77         3.00         4.00
//        4.50         4.05        17.30         3.25         4.50
//        5.00         6.85        14.77         3.50         5.00
//        5.50        10.55        12.94         3.75         5.50
//        6.00        15.27        11.67         4.00         6.00
//        6.50        20.90        10.79         4.25         6.50
//        7.00        27.14        10.15         4.50         7.00
//        7.50        34.03         9.73         4.75         7.50
//        8.00        41.10         9.40         5.00         8.00
//
// %overflow   = percentage of buckets which have an overflow bucket
// bytes/entry = overhead bytes used per key/elem pair
// hitprobe    = # of entries to check when looking up a present key
// missprobe   = # of entries to check when looking up an absent key
//
// Keep in mind this data is for maximally loaded tables, i.e. just
// before the table grows. Typical tables will be somewhat less loaded.
```

##### 3.3.3 初始化

**字面量**

创建的过程和slice基本相同

**运行时**

当我们用 `make(map[k]v)` 或者 `make(map[k]v, hint)`，且 hint 小于等于 8 的时候，会分配到 heap

```golang
// makemap_small implements Go map creation for make(map[k]v) and
// make(map[k]v, hint) when hint is known to be at most bucketCnt
// at compile time and the map needs to be allocated on the heap.
func makemap_small() *hmap {
	h := new(hmap)
	h.hash0 = fastrand()
	return h
}
```

当 hint 比 8 大的时候，调用函数 `makemap` 

```golang
func makemap(t *maptype, hint int, h *hmap) *hmap {
	// 判断是否溢出
	mem, overflow := math.MulUintptr(uintptr(hint), t.bucket.size)
	if overflow || mem > maxAlloc {
		hint = 0
	}

	// initialize Hmap
	if h == nil {
		h = new(hmap)
	}
	h.hash0 = fastrand()

	// Find the size parameter B which will hold the requested # of elements.
	// For hint < 0 overLoadFactor returns false since hint < bucketCnt.
	// 根据传入的 hint 算出最小的 B 值
	B := uint8(0)
	for overLoadFactor(hint, B) {
		B++
	}
	h.B = B

	// allocate initial hash table
	// if B == 0, the buckets field is allocated lazily later (in mapassign)
	// If hint is large zeroing this memory could take a while.
	if h.B != 0 {
		var nextOverflow *bmap
		// 根据 B 值创建桶
		h.buckets, nextOverflow = makeBucketArray(t, h.B, nil)
		if nextOverflow != nil {
			h.extra = new(mapextra)
			h.extra.nextOverflow = nextOverflow
		}
	}

	return h
}
```

##### 读写操作

**访问**

**写入**

**扩容**

扩容有两种情况，第一种就是上面提到的装载因子超过了 6.5
第二种即为使用了太多溢出桶，为等量扩容

```golang
func hashGrow(t *maptype, h *hmap) {
	// If we've hit the load factor, get bigger.
	// Otherwise, there are too many overflow buckets,
	// so keep the same number of buckets and "grow" laterally.
	bigger := uint8(1)
	if !overLoadFactor(h.count+1, h.B) {
		bigger = 0
		h.flags |= sameSizeGrow
	}
	// 将原先的 buckets 转移到 oldbuckets
	oldbuckets := h.buckets
	// 创建新的 buckets 和 溢出桶
	newbuckets, nextOverflow := makeBucketArray(t, h.B+bigger, nil)

	flags := h.flags &^ (iterator | oldIterator)
	if h.flags&iterator != 0 {
		flags |= oldIterator
	}
	// commit the grow (atomic wrt gc)
	h.B += bigger
	h.flags = flags
	h.oldbuckets = oldbuckets
	h.buckets = newbuckets
	h.nevacuate = 0
	h.noverflow = 0

	if h.extra != nil && h.extra.overflow != nil {
		// Promote current overflow buckets to the old generation.
		if h.extra.oldoverflow != nil {
			throw("oldoverflow is not nil")
		}
		h.extra.oldoverflow = h.extra.overflow
		h.extra.overflow = nil
	}
	if nextOverflow != nil {
		if h.extra == nil {
			h.extra = new(mapextra)
		}
		h.extra.nextOverflow = nextOverflow
	}

	// the actual copying of the hash table data is done incrementally
	// by growWork() and evacuate().
}
```

...

**删除**

### 第四章 语言基础

#### 4.1 函数调用

Go 中的参数传递, 无论是传递基本类型、结构体还是**指针**，都会对传递的参数进行拷贝

#### 4.2 接口

##### 4.2.1 概述

**分类**

iface(runtime.iface) - eface(runtime.eface)

**指针和接口**

两种形式，对接口的实现

```golang
// 结构体初始化
func (c  Cat) Quack{}
// 指针初始化
func (c *Cat) Quack{}
```

首先记住一个结论

||结构体实现|结构体指针实现|
|:-:|:-:|:-:|
|结构体初始化变量|P|F|
|结构体指针初始化变量|P|P|

##### 4.2.2 数据结构

上面提到了有两种分类, 其中 `runtime.iface` 表示的是包含方法的接口, `runtime.eface` 表示的是不包含任何方法的 interface{} 类型

```golang
type eface struct {
	typ, val unsafe.Pointer
}

type iface struct {
	tab  *itab
	data unsafe.Pointer
}
```

和书中略微不同的是, `eface` 的这部分 `typ` 也已经变成了 unsafe.Pointer, （猜测可能是因为泛型?）

我们看一下 `itab` 的定义

```golang
type itab struct {
	inter *interfacetype
	_type *_type
	hash  uint32 // copy of _type.hash. Used for type switches.
	_     [4]byte
	fun   [1]uintptr // variable sized. fun[0]==0 means _type does not implement inter.
}
```

fun 这个是动态派发和虚函数表，虚函数表好像在 C++ 里面看过这个，但是忘记了

##### 4.2.2 类型转换

按照书中说的，`go tool compile -S test.go` 一下看汇编

```golang
package main

// go tool compile -S test.go

type Duck interface {
	Quack()
}

type Cat struct {
	Name string
}

//go:noinline
func (c *Cat) Quack() {
	println(c.Name + " meow")
}

func main() {
	var c Duck = &Cat{Name: "draven"}
	c.Quack()
}
```
