package http_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	//"time"

	"github.com/hugomfc/tt/internal/domain"
	ttHttpHandler "github.com/hugomfc/tt/internal/interfaces/http"
	"github.com/hugomfc/tt/internal/ratelimiter"
	"github.com/hugomfc/tt/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRateLimit(t *testing.T) {
  redisStorage, err := storage.NewRedis("127.0.0.1:6379", "", 0, 10*time.Second)

  if err != nil {
    panic(err)
  }

	rl := ratelimiter.New(redisStorage, 1*time.Second, 1)

	rule := domain.Rule{
    ID: "rule1",
		Expression: "Method == 'GET'",
    GroupBy: []string{"ip"},
    Limit: 3,
    WindowSize: 60, 
  }

	rl.AddRule(context.Background(), &rule)

	h := ttHttpHandler.NewHttpHandler(rl)
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "First request should be allowed")

  //time.Sleep(1*time.Second)

	resp, err = http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Second request should be allowed")

  //time.Sleep(1*time.Second)
	resp, err = http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Third request should be allowed")

  //time.Sleep(1*time.Second)
	resp, err = http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "Fourth request should be disallowed")

  //time.Sleep(1*time.Second)
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.NoError(t, err)
  assert.Equal(t, "Too many requests\n", string(body), "Correct error message should be returned")
}
