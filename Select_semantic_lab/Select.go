package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"time"
)

// 如果想做成unbuffered channel，没有container，发了必须的收，太快了，所以压根就不会打印***has been delivered那句话
func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(f)
	}
	defer f.Close()
	trace.Start(f)
	defer trace.Stop()

	kfc_channel := make(chan string, 5)
	mcdonalds_channel := make(chan string, 5)

	// delivery
	go func() {
		time.Sleep(4 * time.Second)
		for i := 1; i <= 2; i++ {
			kfc_channel <- "Beef Burger"
		}
		fmt.Printf("KFC has been delivered.\n")
	}()

	go func() {
		time.Sleep(5 * time.Second)
		for i := 1; i <= 2; i++ {
			mcdonalds_channel <- "McValue"
		}
		fmt.Printf("McDonalds has been delivered.\n")
	}()

	// pickup
	for i := 1; i <= 4; i++ {
		select {
		case kfc := <-kfc_channel:
			fmt.Printf("I have %s\n", kfc)

		case mcdonalds := <-mcdonalds_channel:
			fmt.Printf("I have %s\n", mcdonalds)

		// timeout
		case <-time.After(10 * time.Second):
			fmt.Printf("I have been waiting for so long, I can't wait no more!\n")
			// default:
			// 	fmt.Println("I have something else to do!")
		}
	}

	fmt.Println("I'm so full, I've eat all of them!")
}
