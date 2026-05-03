package main

import (
	"fmt"
	"os"
)

func main() {
	f, _ := os.Create("perf_1m.log")
	defer f.Close()
	for i := 0; i < 1000000; i++ {
		fmt.Fprintf(f, `{"time":"2026-05-03T10:00:00Z","level":"INFO","service":"api","msg":"request-%d","status":200}`+"\n", i)
	}
	fmt.Println("Done! 1,000,000 lines generated in perf_1m.log")
}
