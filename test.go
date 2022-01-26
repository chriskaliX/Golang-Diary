package main

import "fmt"

// go tool compile -S test.go

// type Duck interface {
// 	Quack()
// }

// type Cat struct {
// 	Name string
// }

// //go:noinline
// func (c *Cat) Quack() {
// 	println(c.Name + " meow")
// }

// // slice for-loop
// func test() {
// 	test := []int{1,2,3}
// 	for range test {
// 		fmt.Println("yes")
// 	}

// 	// 会优化, 直接调用 runtime.memclrNoHeapPointers 或者 runtime.memclrHasPointers 清除目标数组内存空间中的全部数据
// 	for i := range test {
// 		test[i] = 0
// 	}
// }

// func main() {
// 	var c Duck = &Cat{Name: "draven"}
// 	c.Quack()
// }

func main() {
	a := []string{"1", "2"}
	a = append(a, []string{"4"}...)
	fmt.Println(cap(a))
}
