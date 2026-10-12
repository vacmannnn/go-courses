package main

import (
	"fmt"
	"testing"
)

// Слайд 32: собираем ответ через append и оцениваем allocation rate.
// Запуск: go run 07_allocation_rate.go
var sink []byte

const (
	payloadSize       = 1024
	requestsPerSecond = 10_000 // Условная нагрузка сервиса.
)

func withoutCapacity() []byte {
	var response []byte // len = cap = 0
	for i := 0; i < payloadSize; i++ {
		response = append(response, byte(i))
	}
	return response
}

func withCapacity() []byte {
	response := make([]byte, 0, payloadSize)
	for i := 0; i < payloadSize; i++ {
		response = append(response, byte(i))
	}
	return response
}

func showRate(name string, build func() []byte) {
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sink = build()
		}
	})

	allocsPerSecond := requestsPerSecond * result.AllocsPerOp()
	mibPerSecond := float64(requestsPerSecond*result.AllocedBytesPerOp()) / (1024 * 1024)
	fmt.Printf("%s: %d allocs/op, %d B/op -> %d allocs/s, %.1f MiB/s\n",
		name, result.AllocsPerOp(), result.AllocedBytesPerOp(), allocsPerSecond, mibPerSecond)
}

func main() {
	fmt.Printf("At %d requests/s, building a %d-byte response with append:\n", requestsPerSecond, payloadSize)
	showRate("without capacity", withoutCapacity)
	showRate("with capacity   ", withCapacity)
}
