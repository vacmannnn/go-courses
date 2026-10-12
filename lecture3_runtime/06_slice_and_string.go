package main

import "fmt"

// копия slice делит backing array с исходным slice;
// string неизменяем через значение типа string.
// Запуск: go run 06_slice_and_string.go
func main() {
	base := []int{1, 2, 3}
	view := base[:2]
	copiedHeader := view
	copiedHeader[0] = 99
	fmt.Println("shared backing array:", base) // [99 2 3]

	limited := view[:2:2] // len=2, cap=2: места для append нет.
	grown := append(limited, 4)
	grown[0] = 7
	fmt.Println("after append: base=", base, "grown=", grown)

	text := "Go"
	bytes := []byte(text)
	bytes[0] = 'N'
	fmt.Println("string=", text, "byte copy=", string(bytes))
}
