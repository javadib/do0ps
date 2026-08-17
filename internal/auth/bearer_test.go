package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/javadib/do0ps/internal/core/domain"
)

// testApp builds a Fiber app that mirrors the composition root wiring: a
// dummy handler guarded by Middleware.
func testApp(t *testing.T, store Store) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Get("/guarded", Middleware(store), func(c fiber.Ctx) error {
		client, ok := ClientFrom(c)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "client missing from context"})
		}
		return c.JSON(fiber.Map{"client_id": client.ID, "client_name": client.Name})
	})
	return app
}

func doRequest(t *testing.T, app *fiber.App, header string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	if header != "" {
		req.Header.Set(fiber.HeaderAuthorization, header)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	return resp
}

func mustStore(t *testing.T, tokens map[string]domain.Client) *StaticStore {
	t.Helper()
	store, err := NewStaticStore(tokens)
	if err != nil {
		t.Fatalf("NewStaticStore() error = %v", err)
	}
	return store
}

func TestMiddlewareValidToken(t *testing.T) {
	store := mustStore(t, map[string]domain.Client{
		"correct-horse-battery-staple": {ID: "client-a", Name: "Team A"},
	})
	app := testApp(t, store)

	resp := doRequest(t, app, "Bearer correct-horse-battery-staple")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	defer resp.Body.Close()
	if string(body) != `{"client_id":"client-a","client_name":"Team A"}` {
		t.Errorf("body = %s, want the authenticated client", body)
	}
}

func TestMiddlewareMissingHeader(t *testing.T) {
	app := testApp(t, mustStore(t, map[string]domain.Client{
		"correct-horse-battery-staple": {ID: "client-a"},
	}))

	resp := doRequest(t, app, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	app := testApp(t, mustStore(t, map[string]domain.Client{
		"correct-horse-battery-staple": {ID: "client-a"},
	}))

	resp := doRequest(t, app, "Bearer wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddlewareMalformedHeader(t *testing.T) {
	app := testApp(t, mustStore(t, map[string]domain.Client{
		"correct-horse-battery-staple": {ID: "client-a"},
	}))

	for _, header := range []string{
		"Basic dXNlcjpwYXNz",
		"Bearer",
		"Bearer ",
		"Bearer   ",
		"bearer",
	} {
		t.Run(header, func(t *testing.T) {
			resp := doRequest(t, app, header)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

func TestParseTokens(t *testing.T) {
	tokens, err := ParseTokens("tok-a:client-a,tok-b:client-b:Team B, tok-c:client-c")
	if err != nil {
		t.Fatalf("ParseTokens() error = %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("len(tokens) = %d, want 3", len(tokens))
	}
	if got := tokens["tok-a"]; got != (domain.Client{ID: "client-a", Name: "client-a"}) {
		t.Errorf("tok-a = %+v, want client-a with default name", got)
	}
	if got := tokens["tok-b"]; got != (domain.Client{ID: "client-b", Name: "Team B"}) {
		t.Errorf("tok-b = %+v, want client-b named Team B", got)
	}
}

func TestParseTokensRejectsMalformed(t *testing.T) {
	for _, spec := range []string{"", "no-colon", "tok-a:", ":client-a", " , "} {
		t.Run(spec, func(t *testing.T) {
			if _, err := ParseTokens(spec); err == nil {
				t.Errorf("ParseTokens(%q) = nil error, want an error", spec)
			}
		})
	}
}

func TestNewStaticStoreRejectsWeakConfig(t *testing.T) {
	if _, err := NewStaticStore(map[string]domain.Client{}); err == nil {
		t.Error("NewStaticStore(empty) = nil error, want an error")
	}
	if _, err := NewStaticStore(map[string]domain.Client{"short": {ID: "client-a"}}); err == nil {
		t.Error("NewStaticStore(short token) = nil error, want an error")
	}
}

func TestLookupConstantTime(t *testing.T) {
	store := mustStore(t, map[string]domain.Client{
		"correct-horse-battery-staple": {ID: "client-a"},
	})

	if _, ok := store.Lookup("correct-horse-battery-staple"); !ok {
		t.Error("Lookup(valid) = false, want true")
	}
	if _, ok := store.Lookup("correct-horse-battery-stapleX"); ok {
		t.Error("Lookup(wrong suffix) = true, want false")
	}
}
