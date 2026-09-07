package posse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Adversarial QA probe (not part of the shipped witness): confirms the two
// "stays allowed" claims in ranger-base-ji1d3's commit message that no
// existing test exercises — an http ask answered by https (upgrade), and a
// same-scheme redirect that only changes port.
func TestQAAdversarialUpgradeAndPortChangeStayAllowed(t *testing.T) {
	t.Parallel()

	t.Run("http_asked_https_answered_is_allowed", func(t *testing.T) {
		var authSeen string
		https := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authSeen = r.Header.Get("Authorization")
			w.Write([]byte("ok"))
		}))
		defer https.Close()
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, https.URL+r.URL.Path, http.StatusFound)
		}))
		defer httpSrv.Close()

		cl := pinnedClient(5*time.Second, "qa upgrade probe")
		cl.Transport = https.Client().Transport
		req, _ := http.NewRequest("GET", httpSrv.URL+"/x", nil)
		req.Header.Set("Authorization", "Bearer qa-upgrade-probe")
		resp, err := cl.Do(req)
		var pin *PinRefusal
		if errors.As(err, &pin) {
			t.Fatalf("http asked, https answered: want no refusal, got %v", err)
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if authSeen == "" {
			t.Errorf("upgrade should still be followed with the header attached; got none")
		}
	})

	t.Run("same_scheme_port_change_is_allowed", func(t *testing.T) {
		var authSeen string
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authSeen = r.Header.Get("Authorization")
			w.Write([]byte("ok"))
		}))
		defer other.Close()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// redirect to a different port on the SAME hostname, same scheme (http)
			target := strings.Replace(other.URL, "127.0.0.1", "127.0.0.1", 1)
			http.Redirect(w, r, target+r.URL.Path, http.StatusFound)
		}))
		defer srv.Close()

		cl := pinnedClient(5*time.Second, "qa port probe")
		req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
		req.Header.Set("Authorization", "Bearer qa-port-probe")
		resp, err := cl.Do(req)
		var pin *PinRefusal
		if errors.As(err, &pin) {
			t.Fatalf("same-scheme port change: want no refusal, got %v", err)
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if authSeen == "" {
			t.Errorf("port change on same scheme/host should still be followed with header attached")
		}
	})
}
