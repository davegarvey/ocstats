package fetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestQueryQuotaSuccess(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != FnQuota {
			t.Errorf("unexpected function id %q", r.URL.Query().Get("id"))
		}
		if r.Header.Get("X-Server-Id") != FnQuota || !strings.HasPrefix(r.Header.Get("X-Server-Instance"), "server-fn:") {
			t.Errorf("missing server headers: %v", r.Header)
		}
		if r.Header.Get("Cookie") != "auth=test-cookie" {
			t.Errorf("missing cookie: %v", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(fixture(t, "success.txt"))
	})
	c := &Client{Base: ts.URL, Cookie: "test-cookie"}
	q, ok, err := c.QueryQuota("wrk_test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected subscription")
	}
	if q.RollingUsage.UsagePercent != 2 || q.WeeklyUsage.ResetInSec != 44104 || q.MonthlyUsage.UsagePercent != 35 {
		t.Fatalf("unexpected quota %+v", q)
	}
	if !q.Mine || q.UseBalance {
		t.Fatalf("unexpected flags %+v", q)
	}
}

func TestQueryQuotaNullSubscription(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(fixture(t, "null-subscription.txt"))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	q, ok, err := c.QueryQuota("wrk_test")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no subscription")
	}
	if q.RollingUsage.UsagePercent != 0 {
		t.Fatalf("expected zeroed quota, got %+v", q)
	}
}

func TestQueryQuotaHTTP500(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(fixture(t, "error500.json"))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	_, _, err := c.QueryQuota("wrk_test")
	if !IsKind(err, KindFetch) {
		t.Fatalf("expected fetch error, got %v", err)
	}
}

func TestQueryQuotaMalformed(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(fixture(t, "malformed.txt"))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	_, _, err := c.QueryQuota("wrk_test")
	if !IsKind(err, KindDecode) {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestQueryQuotaAuthRejected(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-error", "true")
		w.Header().Set("Location", "/auth/authorize")
		w.WriteHeader(http.StatusOK)
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	_, _, err := c.QueryQuota("wrk_test")
	if !IsKind(err, KindAuth) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestResolveWorkspace(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(`;0x0000004e;((self.$R=self.$R||{})["server-fn:12"]=[],"wrk_abc123")`))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	id, err := c.ResolveWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if id != "wrk_abc123" {
		t.Fatalf("got %q", id)
	}
}

func TestResolveWorkspaceNone(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(`;0x00000030;((self.$R=self.$R||{})["server-fn:13"]=[],void 0)`))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	_, err := c.ResolveWorkspace()
	if !IsKind(err, KindNoWorkspace) {
		t.Fatalf("expected no-workspace error, got %v", err)
	}
}

func TestValidateSessionOk(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/status" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Cookie") != "auth=x" {
			t.Errorf("missing cookie: %q", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"account":{"acc_1":{"id":"acc_1","email":"a@b.c"}},"current":"acc_1"}`))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	if err := c.ValidateSession(); err != nil {
		t.Fatalf("session should validate, got %v", err)
	}
}

func TestValidateSessionLoggedOut(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	if err := c.ValidateSession(); !IsKind(err, KindAuth) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestValidateSessionFollowsRotation(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "auth=rotated-seal; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"current":"acc_1"}`))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	if err := c.ValidateSession(); err != nil {
		t.Fatal(err)
	}
	if c.NewCookie != "rotated-seal" {
		t.Fatalf("expected rotated cookie captured, got %q", c.NewCookie)
	}
}

func TestQueryQuotaFollowsRotation(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "auth=rotated2; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write(fixture(t, "success.txt"))
	})
	c := &Client{Base: ts.URL, Cookie: "x"}
	if _, _, err := c.QueryQuota("wrk_test"); err != nil {
		t.Fatal(err)
	}
	if c.NewCookie != "rotated2" {
		t.Fatalf("expected rotated cookie captured, got %q", c.NewCookie)
	}
}
