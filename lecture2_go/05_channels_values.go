package main

import "fmt"

func calculateSum(numbers []int, result chan int) {
	sum := 0

	for _, number := range numbers {
		sum += number
	}

	result <- sum
}

func main() {
	result := make(chan int)

	go calculateSum([]int{1, 2, 3, 4, 5}, result)

	sum := <-result
	fmt.Println("sum from goroutine:", sum)
}
