package main

import (
	"fmt"
	"os"
	"runtime/trace"
)

/**
* Unbuffered channel
* Multiple senders, single receiver
**/
func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	trace.Start(f)
	defer trace.Stop()

	// create a channel that can only transport integer
	channel := make(chan int)

	// assign the tasks to the worker （子goroutine发送）
	for i := 1; i <= 5; i++ {
		go func(id int) {
			fmt.Printf("Worker %d is working right now!\n", id)
			channel <- id // once the work has been done, pass the id into the channel
		}(i)
	}

	// 主goroutine读取
	// The contractor knew he had assigned five people, so he needed to collect five shipments from the pipeline.
	for j := 1; j <= 5; j++ {
		<-channel
	}

	/**
	** single sender(contractor), multiple receiver(workers)
	**/

	// // 5 workers waiting for the tasks
	// for j := 1; j <= 5; j++ {
	// 	go func(id int) {
	// 		taskId := <-channel
	// 		fmt.Printf("Worker %d is working on task %d!\n", id, taskId)
	// 	}(j)

	// }

	// // assign 5 tasks
	// for i := 1; i <= 5; i++ {
	// 	channel <- i
	// }

	// time.Sleep(1 * time.Second)
	fmt.Println("All work has been done!")
}
