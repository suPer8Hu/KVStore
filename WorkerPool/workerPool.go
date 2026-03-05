package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"sync"
)

func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	trace.Start(f)
	defer trace.Stop()

	var wg sync.WaitGroup

	channel := make(chan int)

	/*3个工人抢主协程发的5个任务*/
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for taskId := range channel {
				fmt.Printf("Worker %d is working on the task %d\n", id, taskId)
			}
		}(i)
	}

	// main thread
	for j := 1; j <= 5; j++ {
		channel <- j
	}

	close(channel)

	wg.Wait()
	fmt.Println("All work done!")
}
