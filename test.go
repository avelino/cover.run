package main

import (
	"fmt"
	"math"
	"strconv"
)

func main() {
	percent := 100.555

	out := strconv.FormatFloat(math.Round(percent), 'f', -1, 64)

	fmt.Println(out)
}
