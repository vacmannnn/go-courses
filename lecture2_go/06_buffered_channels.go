package main

import (
	"fmt"
	"time"
)

func main() {
	sem := make(chan int, 2)

	sem <- 1 // заняли слот
	go func() {
		sem <- 2
		fmt.Println("записали 2")
		sem <- 3
		fmt.Println("записали 3")
	}()

	time.Sleep(1 * time.Second)
	fmt.Println(<-sem)
	fmt.Println(<-sem)
	fmt.Println(<-sem)
}
