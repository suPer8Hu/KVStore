package main

import (
	"fmt"
	"sync"
)

/*
* Data race: 账户里有 0 元。派 1000 个工人，每个工人往里存 1 块钱。最后应该有 1000 元
 */
// func main() {
// 	var balance int = 0
// 	var wg sync.WaitGroup
// 	// we need a Mutex lock to deal with the data race
// 	var mu sync.Mutex

// 	for i := 0; i < 1000; i++ {
// 		wg.Add(1)
// 		go func() {
// 			// 防御性编程: 把善后工作（释放资源、解锁、签退）在刚开始就安排好，后面想怎么折腾就怎么折腾
// 			defer wg.Done()
// 			mu.Lock()
// 			defer mu.Unlock()
// 			balance += 1 // worker's behaviour
// 		}()
// 	}

// 	wg.Wait()
// 	fmt.Printf("Final account balance should be: %d\n", balance)
// }

/**
* RWMutex读写锁
* 有 10 个工人负责往账本里存钱（写操作），但同时有 1000 个保安在疯狂地反复查看余额（读操作）
**/
// func main() {
// 	var rwMu sync.RWMutex
// 	var balance int = 0
// 	var wg sync.WaitGroup

// 	for i := 1; i <= 10; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()

// 			rwMu.Lock()
// 			defer rwMu.Unlock()
// 			balance += 100
// 		}()
// 	}

// 	for j := 1; j <= 1000; j++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()

// 			rwMu.RLock()
// 			defer rwMu.RUnlock()
// 			fmt.Printf("I just want to see the balance at a glance! The balance is %d.\n", balance)
// 		}()
// 	}

// 	wg.Wait()
// 	fmt.Printf("Final balance: %d.\n", balance)
// }

/**
* Sync.Once() : 假设你的系统刚启动，有 1000 个用户的请求（1000 个协程）同时涌进来，它们都需要去查询数据库。
但是你的数据库连接池（DB Connection）还没建立。这 1000 个协程一看：“哎呀，没连数据库，我去连一下！”
结果就是：这 1000 个协程同时向数据库发起连接请求，瞬间把数据库打挂了。

实战任务卡：【被抢爆的数据库连接】
现在，搭好了一个 50 个工人同时冲向数据库的场景。
如果直接跑，你会看到满屏的“⚠️ 建立全局数据库连接！”——在现实中，这就意味着你的数据库已经被搞宕机了。
*/

func main() {
	var wg sync.WaitGroup
	var once sync.Once

	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			once.Do(func() {
				fmt.Printf("DB connecting!\n")
			})
			fmt.Printf("Worker %d: DB connected, start working!\n", workerID)
		}(i)
	}

	wg.Wait()
	fmt.Printf("All done!\n")
}
