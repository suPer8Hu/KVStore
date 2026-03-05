package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"sync"
	"time"
)

/*
*
假设你写了一个爬虫程序，要抓取 10,000 个网页。
如果不用并发：一个一个抓，太慢了，要抓到猴年马月。
如果用普通的 Goroutine + WaitGroup：你瞬间 for 循环开出 10,000 个协程。你的电脑可能扛得住，但目标网站的服务器会瞬间被你打死（这叫 DDoS 攻击），你的 IP 立刻就会被封杀。
Task:
有 10 个爬虫任务要执行。
但是目标网站很脆弱，规定同时最多只能有 2 个请求在进行。
利用容量为 2 的 Channel 作为限流器，控制这 10 个任务的执行节奏。
*
*/
func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	trace.Start(f)
	defer trace.Stop()

	var wg sync.WaitGroup
	// create一个 容量为 2 的 Channel 作为限流器（通行证）
	// 在限流器这个场景中，我们完全不关心管道里传递的“具体数据”是什么，我们只关心“坑位”有没有被占满。既然不需要数据，使用 0 字节的 struct{} 来占位是 Go 语言里最标准、最省内存的极致优化写法
	sem := make(chan struct{}, 2)

	// 派10个工人干活
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()
			// 10个工人去抢2个通行证
			// 如果通行证发完了，工人会自动在这里卡住等待
			sem <- struct{}{}
			fmt.Printf("Task %d start...\n", taskID)
			time.Sleep(1 * time.Second)
			fmt.Printf("Task %d complete...\n", taskID)

			// when one is done don't forget to return the ticket back
			<-sem
		}(i)
	}

	wg.Wait()
	fmt.Printf("All 10 tasks has complete!")
}
