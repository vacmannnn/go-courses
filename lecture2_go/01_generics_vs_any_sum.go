package main

import "fmt"

func AnySum(numbers []any) float64 {
	var sum float64

	for _, number := range numbers {
		switch value := number.(type) {
		case int:
			sum += float64(value)
		case float64:
			sum += value
		default:
			fmt.Println("пропускаем не число:", value)
		}
	}

	return sum
}

// Number ограничивает типы, с которыми умеет работать GenericSum.
type Number interface {
	~int | ~int64 | ~float64
}

type MyInt int

func GenericSum[T Number](numbers []T) T {
	var sum T

	for _, number := range numbers {
		sum += number
	}

	return sum
}

func main() {
	anyNumbers := []any{1, 2, 3.5, "oops"}
	fmt.Println("sum with any:", AnySum(anyNumbers))

	intNumbers := []int{1, 2, 3}
	fmt.Println("sum with generics for ints:", GenericSum(intNumbers))

	floatNumbers := []MyInt{1, 20, 3}
	fmt.Println("sum with generics for floats:", GenericSum(floatNumbers))
}
