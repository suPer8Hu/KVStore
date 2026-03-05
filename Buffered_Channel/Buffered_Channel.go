package main

import "fmt"

/**
** Buffered channel
**/
func main() {
	channel := make(chan int, 3) // vol:3 container

	for i := 1; i <= 3; i++ {
		channel <- i
		fmt.Printf("Successfully send the task %d\n", i)
	}

	for j := 1; j <= 3; j++ {
		task := <-channel
		fmt.Printf("Took out task %d\n", task)
	}

	fmt.Println("All done!")
}
