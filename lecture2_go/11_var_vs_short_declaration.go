package main

import "fmt"

func main() {
	var count int      // можно объявить без начального значения
	message := "hello" // короткая запись с выводом типа

	if count == 0 {
		count := 10 // новая переменная только внутри блока
		fmt.Println("inside:", count)
	}

	fmt.Println(message)
	fmt.Println("outside:", count)
}
