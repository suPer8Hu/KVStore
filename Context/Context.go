package main

import (
	"context"
	"fmt"
	"time"
)

/*
** Context cancel: 你雇了一个工人，让他进入一个无限循环，每隔 1 秒钟打印一句 "Worker is working hard..."。
但是，作为包工头（main），你只打算让他干 3 秒钟。3 秒一到，你就要按下红色的 cancel 按钮，强制让他停工。
*/
func main() {
	// create the Walkie-talkie and the cancel button
	walkieTalkie, cancel := context.WithCancel(context.Background())

	// automatic control
	// walkieTalkie, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Off work! Go home now!")
				return
			default:
				fmt.Println("Worker is still hard working...")
				time.Sleep(1 * time.Second)
			}
		}
	}(walkieTalkie)

	// main goroutine
	time.Sleep(3 * time.Second) // let the worker to do the work for 3 sec
	cancel()
	time.Sleep(1 * time.Second)
	// time.Sleep(4 * time.Second)
}
