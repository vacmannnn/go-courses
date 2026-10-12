package main

import (
	"fmt"
	"testing"
)

// сколько heap allocations требует рост slice.
// Запуск: go run 04_slice_allocations.go

var resultSink []int

func withoutCapacity(n int) []int {
	var result []int
	for i := 0; i < n; i++ {
		result = append(result, i)
	}
	return result
}

func withCapacity(n int) []int {
	result := make([]int, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, i)
	}
	return result
}

func main() {
	const n = 1000
	without := testing.AllocsPerRun(100, func() {
		resultSink = withoutCapacity(n)
	})
	with := testing.AllocsPerRun(100, func() {
		resultSink = withCapacity(n)
	})
	fmt.Printf("without capacity: %.0f allocs/run\n", without)
	fmt.Printf("with capacity:    %.0f allocs/run\n", with)
}
