package main

import (
	"fmt"
	"sync"
	"time"
)

/*
* 真实的微服务架构里，大部分 API 的限流规则都是基于时间的，比如：“严格限制每秒最多只能请求 2 次（2 QPS）”
这意味着，无论你瞬间涌进来 10 个还是 10,000 个并发请求，我都必须强制你们每隔 500 毫秒才能放行一个
Token Bucket（令牌桶算法）与 Ticker
Task: 现在，有 5 个瞬间启动的并发任务，要求用 time.Ticker 强行把它们的执行频率压制在 每 500 毫秒 1 个
*/
func main() {
	var wg sync.WaitGroup
	// 这里不再需要make channel了，因为time.Ticker本质上就是一个内部封装了Channel的发牌机
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// 派5个工人去request api
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()
			//必须从发牌机的管道（ticker.C）里拿到系统按时塞进来的数据，才能往下走
			<-ticker.C
			now := time.Now().Format("15:04:05.000")
			fmt.Printf("[%s] task %d complete! \n", now, taskID)
		}(i)
	}

	wg.Wait()
	fmt.Printf("All the work has been completed under 2 QPS!\n")
}
