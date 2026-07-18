package main

import (
	"log"
	"os"
)

func main() {
	if err := runServer("0.0.0.0:6379"); err != nil {
		log.Printf("[Error] %v", err)
		os.Exit(1)
	}
}
