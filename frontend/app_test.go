package frontend

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestAppConstruction(t *testing.T) {
	_, err := newApp(nil, nil, testOptions())
	require.EqualError(t, err, "API client must not be nil")
	_, err = newApp(newFakeAPI(), nil, Options{})
	require.EqualError(t, err, "frontend options are invalid")
	app, err := newApp(newFakeAPI(), slog.New(slog.DiscardHandler), testOptions())
	require.NoError(t, err)
	require.Len(t, app.templates, 7)
	_, err = New("://bad", http.DefaultClient, nil, testOptions())
	require.ErrorContains(t, err, "invalid API base URL")
	app, err = New("http://localhost/api", http.DefaultClient, nil, testOptions())
	require.NoError(t, err)
	require.NotNil(t, app)
}

func TestPublicPages(t *testing.T) {
	client := newFakeAPI()
	app := newTestApp(t, client)
	tests := []struct {
		path string
		text string
	}{
		{path: "/", text: "Typed clients"},
		{path: "/?page=2&tag=go", text: "Popular Tags"},
		{path: "/article/typed-clients", text: "Useful"},
		{path: "/profile/alice", text: "Gopher"},
		{path: "/profile/alice?tab=favorites", text: "Favorited Articles"},
		{path: "/login", text: "Sign in"},
		{path: "/register", text: "Sign up"},
	}
	for _, test := range tests {
		response := request(t, app, http.MethodGet, test.path, nil, false)
		require.Equal(t, http.StatusOK, response.Code, test.path)
		require.Contains(t, response.Body.String(), test.text)
	}
	require.Equal(t, http.StatusNotFound, request(t, app, http.MethodGet, "/missing", nil, false).Code)
	styles := request(t, app, http.MethodGet, "/styles.css", nil, false)
	require.Equal(t, http.StatusOK, styles.Code)
	require.Contains(t, styles.Body.String(), ".navbar")
}

func TestAuthenticatedPages(t *testing.T) {
	client := newFakeAPI()
	app := newTestApp(t, client)
	tests := []struct {
		path string
		text string
	}{
		{path: "/?feed=following", text: "Your Feed"},
		{path: "/settings", text: "alice@example.test"},
		{path: "/editor", text: "Publish Article"},
		{path: "/editor/typed-clients", text: "A generated client keeps SSR honest."},
		{path: "/article/typed-clients", text: "Delete Article"},
		{path: "/profile/alice", text: "Edit Profile Settings"},
	}
	for _, test := range tests {
		response := request(t, app, http.MethodGet, test.path, nil, true)
		require.Equal(t, http.StatusOK, response.Code, test.path)
		require.Contains(t, response.Body.String(), test.text)
	}
	for _, path := range []string{"/login", "/register"} {
		require.Equal(t, http.StatusSeeOther, request(t, app, http.MethodGet, path, nil, true).Code)
	}
}

func TestProtectedPagesRedirect(t *testing.T) {
	app := newTestApp(t, newFakeAPI())
	for _, path := range []string{"/?feed=following", "/settings", "/editor", "/editor/typed-clients"} {
		response := request(t, app, http.MethodGet, path, nil, false)
		require.Equal(t, http.StatusSeeOther, response.Code, path)
		require.Equal(t, "/login", response.Header().Get("Location"))
	}
}

func TestAuthenticationActions(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		form url.Values
		call string
	}{
		{name: "login", path: "/login", form: url.Values{"email": {"alice@example.test"}, "password": {"secret"}}, call: "login"},
		{name: "register", path: "/register", form: url.Values{"username": {"alice"}, "email": {"alice@example.test"}, "password": {"secret"}}, call: "register"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newFakeAPI()
			response := request(t, newTestApp(t, client), http.MethodPost, test.path, test.form, false)
			require.Equal(t, http.StatusSeeOther, response.Code)
			require.Equal(t, 1, client.calls[test.call])
			cookies := response.Result().Cookies()
			require.NotEmpty(t, cookies)
			require.True(t, cookies[0].HttpOnly)
		})
	}
	client := newFakeAPI()
	client.errors["login"] = &apiFailure{Status: http.StatusUnauthorized, Fields: map[string][]string{"email": {"is invalid"}}}
	response := request(t, newTestApp(t, client), http.MethodPost, "/login", url.Values{"email": {"bad"}, "password": {"bad"}}, false)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "email is invalid")

	response = request(t, newTestApp(t, newFakeAPI()), http.MethodPost, "/logout", url.Values{}, true)
	require.Equal(t, http.StatusSeeOther, response.Code)
	require.Equal(t, -1, response.Result().Cookies()[0].MaxAge)
}

func TestMutationActions(t *testing.T) {
	articleValues := url.Values{"title": {"Typed clients"}, "description": {"Description"}, "body": {"Body"}, "tags": {"go, ssr, go"}}
	tests := []struct {
		name     string
		path     string
		form     url.Values
		call     string
		location string
	}{
		{name: "settings", path: "/settings", form: url.Values{"username": {"alice"}, "email": {"alice@example.test"}, "bio": {"Gopher"}, "image": {"https://example.test/a.png"}, "password": {"new-secret"}}, call: "updateUser", location: "/profile/alice"},
		{name: "create article", path: "/editor", form: articleValues, call: "createArticle", location: "/article/typed-clients"},
		{name: "update article", path: "/editor/typed-clients", form: articleValues, call: "updateArticle", location: "/article/typed-clients"},
		{name: "delete article", path: "/article/typed-clients/delete", form: url.Values{}, call: "deleteArticle", location: "/"},
		{name: "favorite", path: "/article/typed-clients/favorite", form: url.Values{"favorite": {"true"}, "return_to": {"/"}}, call: "setFavorite", location: "/"},
		{name: "follow", path: "/profile/bob/follow", form: url.Values{"follow": {"true"}, "return_to": {"/profile/bob"}}, call: "setFollowing", location: "/profile/bob"},
		{name: "create comment", path: "/article/typed-clients/comments", form: url.Values{"body": {"Nice"}}, call: "createComment", location: "/article/typed-clients"},
		{name: "delete comment", path: "/article/typed-clients/comments/7/delete", form: url.Values{}, call: "deleteComment", location: "/article/typed-clients"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newFakeAPI()
			response := request(t, newTestApp(t, client), http.MethodPost, test.path, test.form, true)
			require.Equal(t, http.StatusSeeOther, response.Code)
			require.Equal(t, test.location, response.Header().Get("Location"))
			require.Equal(t, 1, client.calls[test.call])
		})
	}
}

func TestMutationsRequireAuthentication(t *testing.T) {
	app := newTestApp(t, newFakeAPI())
	for _, path := range []string{
		"/settings", "/editor", "/editor/story", "/article/story/delete",
		"/article/story/favorite", "/profile/bob/follow", "/article/story/comments",
		"/article/story/comments/7/delete",
	} {
		response := request(t, app, http.MethodPost, path, url.Values{}, false)
		require.Equal(t, http.StatusSeeOther, response.Code, path)
		require.Equal(t, "/login", response.Header().Get("Location"))
	}
}

func TestMutationAPIErrors(t *testing.T) {
	articleValues := url.Values{"title": {"Title"}, "description": {"Description"}, "body": {"Body"}}
	tests := []struct {
		name       string
		path       string
		form       url.Values
		operation  string
		wantStatus int
	}{
		{name: "register", path: "/register", form: url.Values{"username": {"used"}, "email": {"used@example.test"}, "password": {"secret"}}, operation: "register", wantStatus: http.StatusOK},
		{name: "settings", path: "/settings", form: url.Values{"username": {"alice"}, "email": {"bad"}}, operation: "updateUser", wantStatus: http.StatusOK},
		{name: "create article", path: "/editor", form: articleValues, operation: "createArticle", wantStatus: http.StatusOK},
		{name: "update article", path: "/editor/story", form: articleValues, operation: "updateArticle", wantStatus: http.StatusOK},
		{name: "delete article", path: "/article/story/delete", form: url.Values{}, operation: "deleteArticle", wantStatus: http.StatusBadGateway},
		{name: "favorite", path: "/article/story/favorite", form: url.Values{"favorite": {"true"}}, operation: "setFavorite", wantStatus: http.StatusBadGateway},
		{name: "follow", path: "/profile/bob/follow", form: url.Values{"follow": {"true"}}, operation: "setFollowing", wantStatus: http.StatusBadGateway},
		{name: "create comment", path: "/article/story/comments", form: url.Values{"body": {"bad"}}, operation: "createComment", wantStatus: http.StatusBadGateway},
		{name: "delete comment", path: "/article/story/comments/7/delete", form: url.Values{}, operation: "deleteComment", wantStatus: http.StatusBadGateway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newFakeAPI()
			client.errors[test.operation] = &apiFailure{Status: http.StatusUnprocessableEntity, Fields: map[string][]string{"body": {"is invalid"}}}
			response := request(t, newTestApp(t, client), http.MethodPost, test.path, test.form, test.operation != "register")
			require.Equal(t, test.wantStatus, response.Code)
		})
	}
}

func TestHandlerErrors(t *testing.T) {
	t.Run("expired session", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["currentUser"] = &apiFailure{Status: http.StatusUnauthorized}
		response := request(t, newTestApp(t, client), http.MethodGet, "/settings", nil, true)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, 1, client.calls["refresh"])
		cookies := response.Result().Cookies()
		require.Len(t, cookies, 2)
		require.Equal(t, "refreshed-token", cookies[0].Value)
		require.Equal(t, "rotated-refresh-token", cookies[1].Value)
	})
	t.Run("expired refresh session", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["currentUser"] = &apiFailure{Status: http.StatusUnauthorized}
		client.errors["refresh"] = &apiFailure{Status: http.StatusUnauthorized}
		response := request(t, newTestApp(t, client), http.MethodGet, "/settings", nil, true)
		require.Equal(t, http.StatusSeeOther, response.Code)
		cookies := response.Result().Cookies()
		require.Len(t, cookies, 2)
		require.Equal(t, -1, cookies[0].MaxAge)
		require.Equal(t, -1, cookies[1].MaxAge)
	})
	t.Run("missing article", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["article"] = errNotFound
		require.Equal(t, http.StatusNotFound, request(t, newTestApp(t, client), http.MethodGet, "/article/missing", nil, false).Code)
	})
	t.Run("API unavailable", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["articles"] = errors.New("offline")
		require.Equal(t, http.StatusBadGateway, request(t, newTestApp(t, client), http.MethodGet, "/", nil, false).Code)
	})
	t.Run("current user unavailable", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["currentUser"] = errors.New("offline")
		require.Equal(t, http.StatusBadGateway, request(t, newTestApp(t, client), http.MethodGet, "/", nil, true).Code)
	})
	for _, operation := range []string{"feed", "tags", "comments", "profile"} {
		t.Run(operation+" unavailable", func(t *testing.T) {
			client := newFakeAPI()
			client.errors[operation] = errors.New("offline")
			path := "/"
			authenticated := false
			switch operation {
			case "feed":
				path, authenticated = "/?feed=following", true
			case "comments":
				path = "/article/story"
			case "profile":
				path = "/profile/alice"
			}
			require.Equal(t, http.StatusBadGateway, request(t, newTestApp(t, client), http.MethodGet, path, nil, authenticated).Code)
		})
	}
	t.Run("forbidden editor", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["article"] = &apiFailure{Status: http.StatusForbidden}
		require.Equal(t, http.StatusForbidden, request(t, newTestApp(t, client), http.MethodGet, "/editor/typed-clients", nil, true).Code)
	})
	t.Run("invalid comment id", func(t *testing.T) {
		response := request(t, newTestApp(t, newFakeAPI()), http.MethodPost, "/article/story/comments/bad/delete", url.Values{}, true)
		require.Equal(t, http.StatusNotFound, response.Code)
	})
	t.Run("oversized form", func(t *testing.T) {
		app, err := newApp(newFakeAPI(), nil, Options{CookieName: "session", RefreshCookieName: "refresh", CookieTTL: testOptions().CookieTTL, MaxFormBytes: 1})
		require.NoError(t, err)
		response := request(t, app, http.MethodPost, "/login", url.Values{"email": {"too-large"}}, false)
		require.Equal(t, http.StatusBadRequest, response.Code)
	})
	t.Run("unauthorized mutation", func(t *testing.T) {
		client := newFakeAPI()
		client.errors["deleteArticle"] = &apiFailure{Status: http.StatusUnauthorized}
		response := request(t, newTestApp(t, client), http.MethodPost, "/article/story/delete", url.Values{}, true)
		require.Equal(t, http.StatusSeeOther, response.Code)
		require.Equal(t, -1, response.Result().Cookies()[0].MaxAge)
	})
}

func TestHelpers(t *testing.T) {
	require.Equal(t, []string{"email is invalid", "username is taken"}, formErrors(&apiFailure{Fields: map[string][]string{"username": {"is taken"}, "email": {"is invalid"}}}))
	require.Equal(t, []string{"Unable to complete the request"}, formErrors(errors.New("offline")))
	require.Equal(t, http.StatusConflict, failureStatus(&apiFailure{Status: http.StatusConflict}))
	require.Zero(t, failureStatus(errors.New("offline")))
	require.Equal(t, []string{"go", "ssr"}, splitTags("go, ssr, go, "))
	require.Equal(t, "go, ssr", joinTags([]string{"go", "ssr"}))
	require.Equal(t, 4, pageNumber("4"))
	require.Equal(t, 1, pageNumber("bad"))
	require.Equal(t, []int{1, 2, 3}, pagination(21, 10))
	require.Empty(t, pagination(0, 10))
	response := httptest.NewRecorder()
	redirect(response, "https://evil.example")
	require.Equal(t, "/", response.Header().Get("Location"))
	response = httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(url.Values{"return_to": {"//evil.example"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newTestApp(t, newFakeAPI()).redirectBack(response, req, "/safe")
	require.Equal(t, "/safe", response.Header().Get("Location"))
}

func TestTemplateErrors(t *testing.T) {
	app := newTestApp(t, newFakeAPI())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.render(response, req, "missing", &pageData{})
	require.Equal(t, http.StatusBadGateway, response.Code)
	app.templates["broken"] = template.Must(template.New("broken").Parse(`{{define "layout"}}{{index .Tags 0}}{{end}}`))
	app.render(httptest.NewRecorder(), req, "broken", &pageData{})
	_, err := parseTemplates(fstest.MapFS{})
	require.ErrorContains(t, err, "parse home template")
}
