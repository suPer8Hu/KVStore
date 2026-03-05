package main

import (
	"fmt"
	"sync"
)

/*
*
3 个厨师 (Producers)：每个人负责做 3 个汉堡（总共 9 个），做完塞进传送带。
2 个外卖员 (Consumers)：一直盯着传送带，出来一个汉堡就抢走去送。
传送带 (Channel)：容量为 5 的带缓冲管道。
需要两个 WaitGroup（签到表）。一个管厨师，一个管外卖员。
*
*/
func main() {
	channel := make(chan string, 5)
	var wgProducers sync.WaitGroup // 厨师
	var wgConsumers sync.WaitGroup // 外卖员

	// 3 chief
	for i := 1; i <= 3; i++ {
		wgProducers.Add(1)
		go func(chefID int) {
			// 每个厨师负责做3个burger，做完塞进channel，做完要done
			defer wgProducers.Done()
			for j := 1; j <= 3; j++ { // 3 burger
				channel <- "Burger"
				fmt.Printf("Burger: Chief %d made it\n", chefID)
			}
		}(i)
	}

	// when all producers has finished their jobs, we need to close the channel
	go func() {
		wgProducers.Wait()
		close(channel)
		fmt.Printf("All chief has finished their work, well done! Close the channel now!\n")
	}()

	// Consumer: 2 dashers
	for i := 1; i <= 2; i++ {
		wgConsumers.Add(1)
		go func(dasherID int) {
			defer wgConsumers.Done()
			// 快递员去管道里拿burger，因为之前producers把burger塞进了channel的另一端
			for burger := range channel {
				fmt.Printf("Dasher %d deliver the %v\n", dasherID, burger)
			}
		}(i)
	}

	wgConsumers.Wait()
	fmt.Printf("All the burgers has been delivered! All done!\n")
}
