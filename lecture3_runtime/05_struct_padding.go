package main

import (
	"fmt"
	"unsafe"
)

// порядок полей меняет размер struct из-за выравнивания.
// Запуск: go run 05_struct_padding.go

type Bad struct {
	Flag  bool
	Count int64
	Code  byte
}

type Better struct {
	Count       int64
	SecondCount int32
	Flag        bool
	Code        byte
}

func main() {
	fmt.Printf("Bad: %d bytes; offsets: Flag=%d Count=%d Code=%d\n",
		unsafe.Sizeof(Bad{}),
		unsafe.Offsetof(Bad{}.Flag),
		unsafe.Offsetof(Bad{}.Count),
		unsafe.Offsetof(Bad{}.Code))
	fmt.Printf("Better: %d bytes; offsets: Count=%d Flag=%d Code=%d\n",
		unsafe.Sizeof(Better{}),
		unsafe.Offsetof(Better{}.Count),
		unsafe.Offsetof(Better{}.Flag),
		unsafe.Offsetof(Better{}.Code))
}
