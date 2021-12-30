# Golang-Diary

This repository is for recording my golang learning. It's very important !!! [Reference](https://draveness.me/golang/docs/). PS: The book was bought

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