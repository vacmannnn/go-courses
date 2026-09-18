package main

import "fmt"

func divide(a, b int) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("recovered:", err)
		}
	}()
	defer fmt.Println("divide finished")

	if b == 0 {
		panic("division by zero")
	}

	fmt.Println("result:", a/b)
}

func main() {
	divide(10, 0)
	fmt.Println("program continues")
}
