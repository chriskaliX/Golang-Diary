# Golang-Diary

This repository is for recording my golang learning. It's very important !!! [Reference](https://draveness.me/golang/docs/). PS: The book was bought

> 写了一段时间的 go, 原理\设计等方面的缺失必须要引起重视! 计划为在 2 个月内完成所有部分的阅读(源码级)

Start from: 2021-12-29 23:35

## 预备知识

## 第二部分: 基础知识

### 第三章: 数据结构

#### 3.1 数组

##### 概述

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
    // typeIsShape: represents a set of closely related types, for generics
    if elem.HasShape() {
        t.SetHasShape(true)
    }
    return t
}
```

##### 初始化

初始化的方式有两种，一种固定长度，一种推导。推导在编译时候就会被转换成为固定长度

```go
arr1 := [3]int{1, 2, 3}
arr2 := [...]int{1, 2, 3}
```

###### 上限推导
