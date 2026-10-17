package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HandlerParallel behaves like Handler but runs all checks concurrently, so
// the endpoint's latency is that of the slowest check rather than the sum of
// all of them. Each goroutine writes only its own slot of a pre-sized slice,
// so no mutex is needed; the map is built after wg.Wait.
func HandlerParallel(timeout time.Duration, checks map[string]Check) http.Handler {
	// Freeze the names once at construction time instead of per request.
	names := make([]string, 0, len(checks))
	fns := make([]Check, 0, len(checks))
	for name, c := range checks {
		names = append(names, name)
		fns = append(fns, c)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs := make([]error, len(fns))
		var wg sync.WaitGroup
		wg.Add(len(fns))
		for i, check := range fns {
			go func() {
				defer wg.Done()
				errs[i] = runCheck(r.Context(), timeout, check)
			}()
		}
		wg.Wait()

		resp := Response{Status: "ok", Checks: make(map[string]string, len(names))}
		for i, name := range names {
			if errs[i] != nil {
				resp.Status = "fail"
				resp.Checks[name] = errs[i].Error()
				continue
			}
			resp.Checks[name] = "ok"
		}
		status := http.StatusOK
		if resp.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
