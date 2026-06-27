package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseCSVSet(t *testing.T) {
	got := parseCSVSet(" payment,auth, ,billing ")

	for _, key := range []string{"payment", "auth", "billing"} {
		if !got[key] {
			t.Fatalf("parseCSVSet() missing key %q in %#v", key, got)
		}
	}
	if got[""] {
		t.Fatalf("parseCSVSet() included empty key: %#v", got)
	}
}

func TestParseSubscription(t *testing.T) {
	ctx := newTestGinContext("/v1/realtime/logs?app=payment,auth&level=ERROR,WARN", nil)

	sub := parseSubscription(ctx)

	if !sub.Applications["payment"] || !sub.Applications["auth"] {
		t.Fatalf("Applications = %#v", sub.Applications)
	}
	if !sub.Levels["ERROR"] || !sub.Levels["WARN"] {
		t.Fatalf("Levels = %#v", sub.Levels)
	}
}

func TestParsePrincipal(t *testing.T) {
	headers := map[string]string{
		"X-User-ID":   "user-1",
		"X-User-Role": "engineer",
		"X-User-Apps": "payment,auth",
	}
	ctx := newTestGinContext("/v1/realtime/logs", headers)

	principal := parsePrincipal(ctx)

	if principal.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", principal.UserID)
	}
	if principal.Role != "engineer" {
		t.Fatalf("Role = %q, want engineer", principal.Role)
	}
	if !principal.Apps["payment"] || !principal.Apps["auth"] {
		t.Fatalf("Apps = %#v", principal.Apps)
	}
}

func TestParsePrincipalDefaultsAnonymous(t *testing.T) {
	ctx := newTestGinContext("/v1/realtime/logs", nil)

	principal := parsePrincipal(ctx)

	if principal.UserID != "anonymous" {
		t.Fatalf("UserID = %q, want anonymous", principal.UserID)
	}
	if principal.Role != "anonymous" {
		t.Fatalf("Role = %q, want anonymous", principal.Role)
	}
	if len(principal.Apps) != 0 {
		t.Fatalf("Apps = %#v, want empty", principal.Apps)
	}
}

func newTestGinContext(target string, headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	ctx.Request = req
	return ctx
}
