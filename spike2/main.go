package main

import "fmt"

func init() {
	fmt.Println("Init runs before main")
}

func init() {
	fmt.Println("Second init runs before main but after first init")
}

func main() {
	fmt.Println("Main runs after init")
}
