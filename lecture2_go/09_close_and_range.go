package main

import "fmt"

func sendNumbers(numbers chan<- int) {
	defer close(numbers)

	for number := 1; number <= 3; number++ {
		numbers <- number
	}
}

func main() {
	numbers := make(chan int)

	go sendNumbers(numbers)

	for number := range numbers {
		fmt.Println(number)
	}

	fmt.Println("channel closed")
}
