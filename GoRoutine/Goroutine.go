package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"sync"
)

// func worker(id int, wg *sync.WaitGroup) {
// 	defer wg.Done() // count down
// 	fmt.Printf("Worker %d is working\n", id)
// }

// func main() {
// 	var wg sync.WaitGroup // counter(countup, countdown or wait)

// 	for i := 1; i <= 5; i++ {
// 		wg.Add(1) // count up

// 		// 1) anonymous func
// 		// go func(id int) {
// 		// 	worker(id, &wg)
// 		// }(i)

// 		// 2)
// 		go worker(i, &wg)
// 	}
// 	wg.Wait() // wait the work to be done and clear to 0 task
// 	fmt.Print("All workers are done!")
// }

/*
* 用匿名函数我们就不需要再创造一个函数名，可以直接调用，直接在main里调用，然后用closure的目的是我们就不需要在匿名函数里传指针了
 */
func main() {
	// create the recorder file(output file for trace data)
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// then we gotta start the recorder
	trace.Start(f)
	defer trace.Stop()

	// WaitGroup 类比：包工头拿个本子记数，工人在远处喊“我干完了”，包工头自己划掉数字
	var wg sync.WaitGroup

	for i := 1; i <= 5; i++ {
		wg.Add(1)

		// anonymous func and closure so we don't have to
		// pass in *sync.WaitGroup
		go func(id int) {
			defer wg.Done()
			fmt.Printf("Worker %d is working right now!\n", id)
		}(i)
	}

	wg.Wait()
	fmt.Println("Work done!")
}
