// File deduplication
package main

import (
	"fmt"
	"os"
)

func main_i() int {

	fmt.Println("Hello, dedup!")

	return 0
}

func main() {
	result := main_i()

	os.Exit(result)
}
