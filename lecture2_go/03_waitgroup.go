package main

import (
	"fmt"
	"sync"
	"time"
)

func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Println("worker started:", id)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("worker finished:", id)
}

func main() {
	var wg sync.WaitGroup

	for id := 1; id <= 3; id++ {
		wg.Add(1)
		go worker(id, &wg)
	}

	fmt.Println("main waits for workers")
	wg.Wait()
	fmt.Println("all workers finished")
}
