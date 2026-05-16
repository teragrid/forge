package main

// Canonical fixture: triggers correctness + reliability + compliance + DX scanners.

import (
	"fmt"
	"log"
	"net/http"
)

// TODO: refactor this function before release (triggers DX: todo-fixme-density)
func processOrder(email string, price float64) {
	// correctness: float-money-arithmetic
	total := price * 1.08

	// compliance: pii-in-logs (log call + email field)
	log.Printf("processing order email=%s total=%f", email, total)

	// compliance: hardcoded-region
	region := "us-east-1"
	fmt.Println("deploying to region:", region)

	// reliability: http-no-timeout (http.Get without context/timeout)
	resp, err := http.Get("http://internal-api.example.com/status")
	if err != nil {
		// reliability: panic-on-error
		panic(err)
	}
	defer resp.Body.Close()

	// reliability: untracked-goroutine
	go func() {
		fmt.Println("background job started")
	}()
}

func main() {
	processOrder("user@example.com", 9.99)
}
