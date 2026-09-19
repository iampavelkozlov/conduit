package frontend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	api "conduit/internal/gen/http"

	"github.com/stretchr/testify/require"
)

const (
	userJSON    = `{"user":{"email":"alice@example.test","token":"token","username":"alice","bio":null,"image":null}}`
	profileJSON = `{"profile":{"username":"alice","bio":null,"image":null,"following":false}}`
	articleJSON = `{"article":{"slug":"story","title":"Story","description":"Desc","body":"Body","tagList":[],"createdAt":"2026-09-15T00:00:00Z","updatedAt":"2026-09-15T00:00:00Z","favorited":false,"favoritesCount":0,"author":{"username":"alice","bio":null,"image":null,"following":false}}}`
)

func TestGeneratedAPISuccess(t *testing.T) {
	authorized := 0
	server := httptest.NewServer(successAPIHandler(&authorized))
	defer server.Close()
	client, err := newGeneratedAPI(server.URL, server.Client())
	require.NoError(t, err)
	ctx := t.Context()

	articles, err := client.articles(ctx, articleQuery{Offset: 10, Limit: 5}, "token")
	require.NoError(t, err)
	require.Equal(t, 2, articles.ArticlesCount)
	_, err = client.feed(ctx, 0, 10, "token")
	require.NoError(t, err)
	tags, err := client.tags(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"go"}, tags)
	article, err := client.article(ctx, "story", "token")
	require.NoError(t, err)
	require.Equal(t, "Story", article.Title)
	_, err = client.comments(ctx, "story", "token")
	require.NoError(t, err)
	_, err = client.profile(ctx, "alice", "token")
	require.NoError(t, err)
	_, err = client.login(ctx, "alice@example.test", "secret")
	require.NoError(t, err)
	_, err = client.register(ctx, "alice", "alice@example.test", "secret")
	require.NoError(t, err)
	session, err := client.refresh(ctx, "token", "refresh-token")
	require.NoError(t, err)
	require.Equal(t, "token", session.User.Token)
	require.Equal(t, "refresh-token", session.RefreshToken)
	_, err = client.currentUser(ctx, "token")
	require.NoError(t, err)
	_, err = client.updateUser(ctx, "token", api.UpdateUser{Bio: new("bio")})
	require.NoError(t, err)
	_, err = client.createArticle(ctx, "token", api.NewArticle{Title: "Story", Description: "Desc", Body: "Body"})
	require.NoError(t, err)
	_, err = client.updateArticle(ctx, "story", "token", api.UpdateArticle{Title: new("Story")})
	require.NoError(t, err)
	require.NoError(t, client.deleteArticle(ctx, "story", "token"))
	require.NoError(t, client.setFavorite(ctx, "story", "token", true))
	require.NoError(t, client.setFavorite(ctx, "story", "token", false))
	require.NoError(t, client.setFollowing(ctx, "bob", "token", true))
	require.NoError(t, client.setFollowing(ctx, "bob", "token", false))
	require.NoError(t, client.createComment(ctx, "story", "Nice", "token"))
	require.NoError(t, client.deleteComment(ctx, "story", 7, "token"))
	require.Positive(t, authorized)
	require.Empty(t, tokenEditors(""))
}

func successAPIHandler(authorized *int) http.Handler {
	type response struct {
		status int
		body   string
	}
	responses := map[string]response{
		"GET /articles/feed":                {body: `{"articles":[],"articlesCount":0}`},
		"GET /articles":                     {body: `{"articles":[],"articlesCount":2}`},
		"POST /articles":                    {status: http.StatusCreated, body: articleJSON},
		"GET /articles/story":               {body: articleJSON},
		"PUT /articles/story":               {body: articleJSON},
		"DELETE /articles/story":            {status: http.StatusNoContent},
		"GET /articles/story/comments":      {body: `{"comments":[]}`},
		"POST /articles/story/comments":     {status: http.StatusCreated, body: `{"comment":{"id":7,"createdAt":"2026-09-15T00:00:00Z","updatedAt":"2026-09-15T00:00:00Z","body":"Nice","author":{"username":"alice","bio":null,"image":null,"following":false}}}`},
		"DELETE /articles/story/comments/7": {status: http.StatusNoContent},
		"POST /articles/story/favorite":     {body: articleJSON},
		"DELETE /articles/story/favorite":   {body: articleJSON},
		"GET /profiles/alice":               {body: profileJSON},
		"POST /profiles/bob/follow":         {body: profileJSON},
		"DELETE /profiles/bob/follow":       {body: profileJSON},
		"POST /users/login":                 {body: userJSON},
		"POST /users":                       {status: http.StatusCreated, body: userJSON},
		"POST /internal/auth/refresh":       {body: userJSON},
		"GET /user":                         {body: userJSON},
		"PUT /user":                         {body: userJSON},
		"GET /tags":                         {body: `{"tags":["go"]}`},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") == "Token token" {
			*authorized++
		}
		if r.URL.Path == "/users/login" || r.URL.Path == "/users" || r.URL.Path == "/internal/auth/refresh" {
			http.SetCookie(w, &http.Cookie{Name: refreshTokenCookieName, Value: "refresh-token"})
		}
		value, ok := responses[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if value.status != 0 {
			w.WriteHeader(value.status)
		}
		_, _ = io.WriteString(w, value.body)
	})
}

func TestGeneratedAPIFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"body":["is invalid"]}}`)
	}))
	defer server.Close()
	client, err := newGeneratedAPI(server.URL, server.Client())
	require.NoError(t, err)
	for _, call := range apiCalls(client, t.Context()) {
		callErr := call()
		require.Error(t, callErr)
		require.Equal(t, http.StatusUnprocessableEntity, failureStatus(callErr))
	}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, call := range apiCalls(client, canceled) {
		require.Error(t, call())
	}

	require.ErrorIs(t, responseFailure("get", &http.Response{StatusCode: http.StatusNotFound}, nil), errNotFound)
	require.ErrorContains(t, responseFailure("get", nil, nil), "response has no status")
	_, err = newGeneratedAPI("://bad", http.DefaultClient)
	require.ErrorContains(t, err, "invalid API base URL")
}

func apiCalls(client *generatedAPI, ctx context.Context) []func() error {
	return []func() error{
		func() error { _, err := client.articles(ctx, articleQuery{Limit: 10}, "token"); return err },
		func() error { _, err := client.feed(ctx, 0, 10, "token"); return err },
		func() error { _, err := client.tags(ctx); return err },
		func() error { _, err := client.article(ctx, "story", "token"); return err },
		func() error { _, err := client.comments(ctx, "story", "token"); return err },
		func() error { _, err := client.profile(ctx, "alice", "token"); return err },
		func() error { _, err := client.login(ctx, "email", "password"); return err },
		func() error { _, err := client.register(ctx, "user", "email", "password"); return err },
		func() error { _, err := client.refresh(ctx, "token", "refresh"); return err },
		func() error { _, err := client.currentUser(ctx, "token"); return err },
		func() error { _, err := client.updateUser(ctx, "token", api.UpdateUser{}); return err },
		func() error { _, err := client.createArticle(ctx, "token", api.NewArticle{}); return err },
		func() error { _, err := client.updateArticle(ctx, "story", "token", api.UpdateArticle{}); return err },
		func() error { return client.deleteArticle(ctx, "story", "token") },
		func() error { return client.setFavorite(ctx, "story", "token", true) },
		func() error { return client.setFavorite(ctx, "story", "token", false) },
		func() error { return client.setFollowing(ctx, "bob", "token", true) },
		func() error { return client.setFollowing(ctx, "bob", "token", false) },
		func() error { return client.createComment(ctx, "story", "body", "token") },
		func() error { return client.deleteComment(ctx, "story", 7, "token") },
	}
}
