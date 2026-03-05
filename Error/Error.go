package main

import "fmt"

/*
人为制造车祸，并在最后一秒抢救成功: defer, panic, recover
*/
func riskyJob() {
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Oh! we actually save this! %s\n", err)
		}
	}()

	fmt.Println("Workers are all working!")
	fmt.Println("They are digging the hole!")

	panic("Boom! Workers dig the high voltage zone, the field is gonna explode!")
}

func main() {
	fmt.Println("Start working!")
	riskyJob()
	fmt.Println("everyone alive!!!")
}
