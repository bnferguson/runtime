package waf

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(slog.Default())
}

func TestBlocksSQLInjection(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h, err := e.Handler(1, backend)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "http://example.com/?id=1%20OR%201%3D1--", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestBlocksXSS(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h, err := e.Handler(1, backend)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "http://example.com/?q=%3Cscript%3Ealert(%27xss%27)%3C/script%3E", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAllowsCleanRequests(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	h, err := e.Handler(1, backend)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestBlocksPostSQLInjection(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h, err := e.Handler(1, backend)
	require.NoError(t, err)

	body := strings.NewReader("username=admin'--&password=x")
	req := httptest.NewRequest("POST", "http://example.com/login", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestParanoiaLevels(t *testing.T) {
	e := testEngine(t)

	for _, level := range []int{1, 2, 3, 4} {
		t.Run("level"+string(rune('0'+level)), func(t *testing.T) {
			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			h, err := e.Handler(level, backend)
			require.NoError(t, err)

			req := httptest.NewRequest("GET", "http://example.com/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestInvalidParanoiaLevel(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	_, err := e.Handler(0, backend)
	assert.Error(t, err)

	_, err = e.Handler(5, backend)
	assert.Error(t, err)
}

func TestEnginesCached(t *testing.T) {
	e := testEngine(t)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	_, err := e.Handler(1, backend)
	require.NoError(t, err)

	assert.Len(t, e.engines, 1)

	_, err = e.Handler(1, backend)
	require.NoError(t, err)

	assert.Len(t, e.engines, 1)

	_, err = e.Handler(2, backend)
	require.NoError(t, err)

	assert.Len(t, e.engines, 2)
}
