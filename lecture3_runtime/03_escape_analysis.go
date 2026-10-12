package main

import "fmt"

// Сравните диагностику для NewCounter и Inc:
// go run -gcflags="-m=2" 03_escape_analysis.go
// noinline не даёт встроить вызовы и скрыть различие между примерами.

var counter *int

//go:noinline
func NewCounter() *int {
	value := 0
	return &value // Адрес должен оставаться валидным после возврата.
}

//go:noinline
func Inc() int {
	value := 0
	p := &value
	(*p)++
	return value // Указатель не покидает вызов.
}

func main() {
	counter = NewCounter()
	fmt.Println("counter:", *counter, "inc:", Inc())
}
