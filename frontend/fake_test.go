package frontend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	api "conduit/internal/gen/http"

	"github.com/stretchr/testify/require"
)

type fakeAPI struct {
	errors map[string]error
	calls  map[string]int
	user   api.User
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		errors: make(map[string]error), calls: make(map[string]int),
		user: api.User{Username: "alice", Email: "alice@example.test", Token: "token", Bio: new("Gopher"), Image: new("https://example.test/alice.png")},
	}
}

func (f *fakeAPI) call(name string) error {
	f.calls[name]++
	return f.errors[name]
}

func (f *fakeAPI) articles(ctx context.Context, _ articleQuery, _ string) (*api.MultipleArticlesResponse, error) {
	if err := f.call("articles"); err != nil {
		return nil, err
	}
	return articleList(ctx), nil
}

func (f *fakeAPI) feed(ctx context.Context, _, _ int, _ string) (*api.MultipleArticlesResponse, error) {
	if err := f.call("feed"); err != nil {
		return nil, err
	}
	return articleList(ctx), nil
}

func (f *fakeAPI) tags(context.Context) ([]string, error) {
	if err := f.call("tags"); err != nil {
		return nil, err
	}
	return []string{"go", "web"}, nil
}

func (f *fakeAPI) article(context.Context, string, string) (*api.Article, error) {
	if err := f.call("article"); err != nil {
		return nil, err
	}
	return fullArticle(), nil
}

func (f *fakeAPI) comments(context.Context, string, string) ([]api.Comment, error) {
	if err := f.call("comments"); err != nil {
		return nil, err
	}
	return []api.Comment{{Id: 7, Author: profile("alice"), Body: "Useful", CreatedAt: testTime()}}, nil
}

func (f *fakeAPI) profile(_ context.Context, username, _ string) (*api.Profile, error) {
	if err := f.call("profile"); err != nil {
		return nil, err
	}
	value := profile(username)
	return &value, nil
}

func (f *fakeAPI) login(context.Context, string, string) (*authSession, error) {
	if err := f.call("login"); err != nil {
		return nil, err
	}
	return &authSession{User: &f.user, RefreshToken: "refresh-token"}, nil
}

func (f *fakeAPI) register(context.Context, string, string, string) (*authSession, error) {
	if err := f.call("register"); err != nil {
		return nil, err
	}
	return &authSession{User: &f.user, RefreshToken: "refresh-token"}, nil
}

func (f *fakeAPI) refresh(context.Context, string, string) (*authSession, error) {
	if err := f.call("refresh"); err != nil {
		return nil, err
	}
	user := f.user
	user.Token = "refreshed-token"
	return &authSession{User: &user, RefreshToken: "rotated-refresh-token"}, nil
}

func (f *fakeAPI) currentUser(context.Context, string) (*api.User, error) {
	if err := f.call("currentUser"); err != nil {
		return nil, err
	}
	return &f.user, nil
}

func (f *fakeAPI) updateUser(context.Context, string, api.UpdateUser) (*api.User, error) {
	if err := f.call("updateUser"); err != nil {
		return nil, err
	}
	return &f.user, nil
}

func (f *fakeAPI) createArticle(context.Context, string, api.NewArticle) (*api.Article, error) {
	if err := f.call("createArticle"); err != nil {
		return nil, err
	}
	return fullArticle(), nil
}

func (f *fakeAPI) updateArticle(context.Context, string, string, api.UpdateArticle) (*api.Article, error) {
	if err := f.call("updateArticle"); err != nil {
		return nil, err
	}
	return fullArticle(), nil
}

func (f *fakeAPI) deleteArticle(context.Context, string, string) error {
	return f.call("deleteArticle")
}

func (f *fakeAPI) setFavorite(context.Context, string, string, bool) error {
	return f.call("setFavorite")
}

func (f *fakeAPI) setFollowing(context.Context, string, string, bool) error {
	return f.call("setFollowing")
}

func (f *fakeAPI) createComment(context.Context, string, string, string) error {
	return f.call("createComment")
}

func (f *fakeAPI) deleteComment(context.Context, string, int, string) error {
	return f.call("deleteComment")
}

func newTestApp(t *testing.T, client conduitAPI) *App {
	t.Helper()
	app, err := newApp(client, nil, testOptions())
	require.NoError(t, err)
	return app
}

func request(t *testing.T, app *App, method, target string, form url.Values, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authenticated {
		req.AddCookie(&http.Cookie{Name: testOptions().CookieName, Value: "token"})
		req.AddCookie(&http.Cookie{Name: testOptions().RefreshCookieName, Value: "refresh-token"})
	}
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)
	return response
}

func testOptions() Options {
	return Options{CookieName: "conduit_session", RefreshCookieName: "conduit_refresh", CookieTTL: time.Minute, MaxFormBytes: 1 << 20}
}

func articleList(ctx context.Context) *api.MultipleArticlesResponse {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"articles":[{"slug":"typed-clients","title":"Typed clients","description":"Compile-time API safety","tagList":["go"],"createdAt":"2026-09-15T00:00:00Z","updatedAt":"2026-09-15T00:00:00Z","favorited":false,"favoritesCount":3,"author":{"username":"alice","bio":"Gopher","image":null,"following":false}}],"articlesCount":12}`))
	}))
	defer server.Close()
	client, _ := api.NewClientWithResponses(server.URL)
	response, _ := client.GetArticlesWithResponse(ctx, nil)
	return response.JSON200
}

func fullArticle() *api.Article {
	return &api.Article{
		Author: profile("alice"), Body: "A generated client keeps SSR honest.", CreatedAt: testTime(),
		Description: "Compile-time API safety", FavoritesCount: 3, Slug: "typed-clients",
		TagList: []string{"go", "ssr"}, Title: "Typed clients", UpdatedAt: testTime(),
	}
}

func profile(username string) api.Profile {
	return api.Profile{Username: username, Bio: new("Gopher"), Image: new("https://example.test/avatar.png")}
}

func testTime() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
