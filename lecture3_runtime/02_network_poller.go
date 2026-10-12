package main

import (
	"fmt"
	"io"
	"net"
)

// горутины ждут данные на TCP-сокетах и продолжают работу
// после готовности сокета. Сам network poller напрямую здесь не виден.
// Запуск: go run 02_network_poller.go
func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	const clients = 4
	ready := make(chan struct{}, clients)
	results := make(chan string, clients)

	go func() {
		for id := 1; id <= clients; id++ {
			conn, err := listener.Accept()
			if err != nil {
				panic(err)
			}
			go func(id int, conn net.Conn) {
				defer conn.Close()
				ready <- struct{}{}
				var data [1]byte
				if _, err := io.ReadFull(conn, data[:]); err != nil {
					results <- fmt.Sprintf("reader %d: %v", id, err)
					return
				}
				results <- fmt.Sprintf("reader %d resumed: %q", id, data[0])
			}(id, conn)
		}
	}()

	connections := make([]net.Conn, 0, clients)
	for range clients {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		connections = append(connections, conn)
	}
	for range clients {
		<-ready
	}
	fmt.Println("TCP readers are waiting; sending one byte to each")
	for _, conn := range connections {
		if _, err := conn.Write([]byte{'G'}); err != nil {
			panic(err)
		}
	}
	for range clients {
		fmt.Println(<-results)
	}
}
