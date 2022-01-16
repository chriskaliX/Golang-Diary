package designpattern

import (
	"fmt"
	"testing"
)

func TestGenerator(t *testing.T) {
	fib := Generator(100)
	for value := range fib {
		fmt.Println(value)
	}
}
