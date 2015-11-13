package main

import (
	"centrifugo-cli"
	"log"
	"time"
)

func main() {
	c := centrifugo.NewClient("ws://172.30.48.215:8000/connection/websocket")
	err := c.Connect()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected")
	time.Sleep(3 * time.Second)
}
