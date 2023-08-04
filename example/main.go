package main

import (
	"fmt"
	"net/http"

	engineIO "github.com/ghuvrons/go.engine.io"
)

type socketHandler struct {
	*engineIO.Socket
}

func main() {
	server := engineIO.NewServer(engineIO.Options{
		PingTimeout:  5000,
		PingInterval: 60000,
	})
	server.OnConnection(func(s *engineIO.Socket) {
		socketHandler := &socketHandler{
			Socket: s,
		}
		s.OnMessage(socketHandler.onGetMessage)
	})

	http.Handle("/engine.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./client")))

	err := http.ListenAndServe(":3333", nil)
	fmt.Println(err)
}

func (socket *socketHandler) onGetMessage(data interface{}) {
	fmt.Println("<", data)
}
