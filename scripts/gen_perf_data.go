//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
)

func main() {
	f, err := os.Create("perf_1m.log")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	for i := 0; i < 1000000; i++ {

		fmt.Fprintf(f, `{"time":"2026-05-04T12:00:00Z","level":"INFO","service":"api","msg":"processed-request-%d","status":200,"latency_ms":12.5,"trace_id":"tr-abcdef-%d"}`+"\n", i, i)
	}
	fmt.Println("Done! 1,000,000 lines generated in perf_1m.log")
}
