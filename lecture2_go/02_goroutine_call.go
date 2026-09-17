package main

import (
	"fmt"
	"time"
)

func say(message string) {
	for i := 1; i <= 3; i++ {
		fmt.Println(message, i)
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	go say("goroutine")

	say("main")

	time.Sleep(300 * time.Millisecond)
	fmt.Println("program finished")
}
