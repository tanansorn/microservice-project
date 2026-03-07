package circuitbreaker

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
)

type BreakerResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func NewBreaker(name string) *gobreaker.CircuitBreaker[BreakerResponse] {
	return gobreaker.NewCircuitBreaker[BreakerResponse](gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("[circuit-breaker] %s: %s -> %s\n", name, from.String(), to.String())
		},
	})
}

func Do(cb *gobreaker.CircuitBreaker[BreakerResponse], req *http.Request) (BreakerResponse, error) {
	return cb.Execute(func() (BreakerResponse, error) {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return BreakerResponse{}, err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 500 {
			return BreakerResponse{
				StatusCode: resp.StatusCode,
				Body:       body,
				Header:     resp.Header,
			}, fmt.Errorf("server error: %d", resp.StatusCode)
		}

		return BreakerResponse{
			StatusCode: resp.StatusCode,
			Body:       body,
			Header:     resp.Header,
		}, nil
	})
}
