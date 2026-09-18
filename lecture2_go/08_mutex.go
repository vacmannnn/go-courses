package main

import (
	"fmt"
	"sync"
)

func main() {
	var (
		counter int
		mutex   sync.Mutex
		wg      sync.WaitGroup
	)

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			mutex.Lock()
			counter++
			mutex.Unlock()
		}()
	}

	wg.Wait()
	fmt.Println("counter:", counter)
}
