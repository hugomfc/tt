package http_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hugomfc/tt/internal/domain"
	ttHttpHandler "github.com/hugomfc/tt/internal/interfaces/http"
	"github.com/hugomfc/tt/internal/ratelimiter"
	"github.com/hugomfc/tt/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRateLimit(t *testing.T) {
	redisStorage, err := storage.NewRedis("127.0.0.1:6379", "", 0, 3*time.Second)
	if err != nil {
		panic(err)
	}

	rl := ratelimiter.New(redisStorage, 1*time.Second, 1)

	defer func() {
		// Tear down the test
		rl.Shutdown()
		redisStorage.Client.FlushDB(context.Background())
		redisStorage.Client.Close()
	}()

	h := ttHttpHandler.NewHttpHandler(rl)
	ts := httptest.NewServer(h)
	defer ts.Close()

	rules := []domain.Rule{
		{
			ID:             "rule1",
			Expression:     "Method == 'GET'",
			GroupBy:        []string{"ip"},
			Limit:          3,
			CountingPeriod: 10,
		},
		{
			ID:             "rule2",
			Expression:     "Method == 'POST'",
			GroupBy:        []string{"ip"},
			Limit:          2,
			CountingPeriod: 10,
		},
	}

	for _, rule := range rules {
		err := rl.AddRule(context.Background(), rule)
		if err != nil {
			panic(err)
		}
	}

	tests := []struct {
		name               string
		method             string
		expected           int
		sleepBeforeRequest time.Duration
	}{
		{
			name:               "First request should be allowed",
			method:             "GET",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Second request should be allowed",
			method:             "GET",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Third request should be allowed",
			method:             "GET",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Fourth request should be disallowed",
			method:             "GET",
			expected:           http.StatusTooManyRequests,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Fifth request should be disallowed",
			method:             "GET",
			expected:           http.StatusTooManyRequests,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Sixth request should be allowed (different method)",
			method:             "POST",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Seventh request should be allowed (different method)",
			method:             "POST",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Eighth request should be disallowed (different method)",
			method:             "POST",
			expected:           http.StatusTooManyRequests,
			sleepBeforeRequest: 0,
		},
		{
			name:               "Nineth request should be allowed (new counting period)",
			method:             "GET",
			expected:           http.StatusOK,
			sleepBeforeRequest: 5 * time.Second,
		},
		{
			name:               "Tenth request should be allowed (new counting period, different method)",
			method:             "POST",
			expected:           http.StatusOK,
			sleepBeforeRequest: 0,
		},
	}

	for _, tt := range tests {
		//_ = redisStorage.Client.FlushAll(context.Background())
		t.Run(tt.name, func(t *testing.T) {
			time.Sleep(tt.sleepBeforeRequest) // Sleep if needed.
			url := fmt.Sprintf("%s/%s", ts.URL, tt.method)
			var resp *http.Response
			var err error
			if tt.method == "GET" {
				resp, err = http.Get(url)
			} else if tt.method == "POST" {
				resp, err = http.Post(url, "application/json", nil)
			} else {
				panic("Invalid HTTP method")
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.StatusCode)
			if resp.StatusCode == http.StatusTooManyRequests {
				body, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				assert.NoError(t, err)
				assert.Equal(t, "Too many requests\n", string(body))
			}
		})
	}
}

func BenchmarkServeHTTP(b *testing.B) {
	redisStorage, err := storage.NewRedis("127.0.0.1:6379", "", 0, 3*time.Second)
	if err != nil {
		panic(err)
	}

	rl := ratelimiter.New(redisStorage, 1*time.Second, 1)

	defer func() {
		// Tear down the test
		rl.Shutdown()
		redisStorage.Client.FlushDB(context.Background())
		redisStorage.Client.Close()
	}()

	// Create a new Handler
	h := ttHttpHandler.NewHttpHandler(rl)
	ts := httptest.NewServer(h)
	defer ts.Close()

	rules := []domain.Rule{
		{
			ID:             "rule1",
			Expression:     "Method == 'GET'",
			GroupBy:        []string{"ip"},
			Limit:          3,
			CountingPeriod: 100,
		},
		{
			ID:             "rule2",
			Expression:     "Method == 'POST'",
			GroupBy:        []string{"ip"},
			Limit:          2,
			CountingPeriod: 100,
		},
	}

	for _, rule := range rules {
		err := rl.AddRule(context.Background(), rule)
		if err != nil {
			panic(err)
		}
	}

	// Start a new HTTP server
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Run the requests concurrently
	b.Run("requests", func(b *testing.B) {
		var wg sync.WaitGroup
		b.Log("Number of requests: ", b.N)
		for i := 0; i < b.N; i++ {
			wg.Add(1)
			go func(i int) {
				// Create a new HTTP client
				client := &http.Client{}
				time.Sleep(1 * time.Second)

				if i%2 == 0 {
					req, _ := http.NewRequest("GET", srv.URL+"/get", nil)
					client.Do(req)
				} else {
					req, _ := http.NewRequest("POST", srv.URL+"/post", nil)
					client.Do(req)
				}

				wg.Done()
			}(i)
		}
		wg.Wait()
	})
}
