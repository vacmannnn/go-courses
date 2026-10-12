package main

import (
	"fmt"
	"runtime"
	"sync"
)

// горутины делят доступные P; порядок их шагов не гарантирован.
// Запуск: go run 01_scheduler.go
func main() {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	start := make(chan struct{})
	var workers sync.WaitGroup

	for id := 1; id <= 3; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			<-start
			for step := 1; step <= 3; step++ {
				fmt.Printf("goroutine %d: step %d\n", id, step)
				runtime.Gosched() // Уступить выполнение, не задавая порядок.
			}
		}(id)
	}

	close(start)
	workers.Wait()
	fmt.Println("all goroutines finished")
}
