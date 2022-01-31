package designpattern

import "fmt"

type Subject struct {
	observers []Observer
	context   string
}

// add observer to list
func (s *Subject) Attach(o Observer) {
	s.observers = append(s.observers, o)
}

// notify the list by calling observer's Update function
func (s *Subject) notify() {
	for _, o := range s.observers {
		o.Update(s)
	}
}

// update and notify
func (s *Subject) UpdateContext(context string) {
	s.context = context
	s.notify()
}

type Observer interface {
	Update(*Subject)
}

type Reader struct {
	name string
}

func (r *Reader) Update(s *Subject) {
	fmt.Printf("%s receive %s\n", r.name, s.context)
}

func NewSubject() *Subject {
	return &Subject{
		observers: make([]Observer, 0),
	}
}

func NewReader(name string) *Reader {
	return &Reader{
		name: name,
	}
}
