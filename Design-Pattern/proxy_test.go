package designpattern

import "testing"

func TestProxy(t *testing.T) {
	var sub ProxySubject
	sub = &Proxy{}
	res := sub.Do()
	if res != "pre:real:after" {
		t.Fail()
	}
}
