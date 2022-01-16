package designpattern

import (
	"fmt"
	"testing"
)

func TestFactory(t *testing.T) {
	db1 := DatabaseFactory("prd")
	db2 := DatabaseFactory("dev")

	db1.PutData("test", "this is mongo")
	db2.PutData("test", "this is sqlite")
	fmt.Println(db1.GetData("test"))
	fmt.Println(db2.GetData("test"))
}
