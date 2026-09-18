package main

import (
	"fmt"
	"sync"
)

func worker(done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	<-done
	fmt.Println("worker stopped")
}

func main() {
	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go worker(done, &wg)

	// Без сигнала done горутина продолжит ждать и станет утечкой.
	close(done)
	wg.Wait()
}
