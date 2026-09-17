package main

import (
	"fmt"
	"time"
)

func slowTask() {
	for i := 1; i <= 5; i++ {
		fmt.Println("slow task step:", i)
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	go slowTask()

	fmt.Println("main goroutine is also a goroutine")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("main returns, program stops with all goroutines")
}
