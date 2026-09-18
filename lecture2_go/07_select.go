package main

import (
	"fmt"
	"time"
)

func main() {
	result := make(chan string)

	go func() {
		time.Sleep(100 * time.Millisecond)
		result <- "готово"
	}()

	select {
	case message := <-result:
		fmt.Println(message)
	case <-time.After(200 * time.Millisecond):
		fmt.Println("время вышло")
}
