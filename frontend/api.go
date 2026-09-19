package frontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	api "conduit/internal/gen/http"
)

var errNotFound = errors.New("resource not found")

type apiFailure struct {
	Status int
	Fields map[string][]string
}

func (e *apiFailure) Error() string { return fmt.Sprintf("Conduit API returned HTTP %d", e.Status) }

type articleQuery struct {
	Tag       *string
	Author    *string
	Favorited *string
	Offset    int
	Limit     int
}

type authSession struct {
	User         *api.User
	RefreshToken string
}

type conduitAPI interface {
	articles(context.Context, articleQuery, string) (*api.MultipleArticlesResponse, error)
	feed(context.Context, int, int, string) (*api.MultipleArticlesResponse, error)
	tags(context.Context) ([]string, error)
	article(context.Context, string, string) (*api.Article, error)
	comments(context.Context, string, string) ([]api.Comment, error)
	profile(context.Context, string, string) (*api.Profile, error)
	login(context.Context, string, string) (*authSession, error)
	register(context.Context, string, string, string) (*authSession, error)
	refresh(context.Context, string, string) (*authSession, error)
	currentUser(context.Context, string) (*api.User, error)
	updateUser(context.Context, string, api.UpdateUser) (*api.User, error)
	createArticle(context.Context, string, api.NewArticle) (*api.Article, error)
	updateArticle(context.Context, string, string, api.UpdateArticle) (*api.Article, error)
	deleteArticle(context.Context, string, string) error
	setFavorite(context.Context, string, string, bool) error
	setFollowing(context.Context, string, string, bool) error
	createComment(context.Context, string, string, string) error
	deleteComment(context.Context, string, int, string) error
}

type generatedAPI struct {
	client         api.ClientWithResponsesInterface
	httpClient     *http.Client
	refreshRequest *http.Request
}

func newGeneratedAPI(baseURL string, httpClient *http.Client) (*generatedAPI, error) {
	parsedURL, err := url.ParseRequestURI(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid API base URL %q", baseURL)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client, err := api.NewClientWithResponses(baseURL, api.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create OpenAPI client: %w", err)
	}
	refreshURL := &url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   strings.TrimSuffix(strings.TrimSuffix(parsedURL.Path, "/"), "/api") + "/internal/auth/refresh",
	}
	refreshRequest := &http.Request{Method: http.MethodPost, URL: refreshURL, Header: make(http.Header)}
	return &generatedAPI{client: client, httpClient: httpClient, refreshRequest: refreshRequest}, nil
}

func (c *generatedAPI) articles(ctx context.Context, query articleQuery, token string) (*api.MultipleArticlesResponse, error) {
	params := &api.GetArticlesParams{Tag: query.Tag, Author: query.Author, Favorited: query.Favorited, Offset: &query.Offset, Limit: &query.Limit}
	response, err := c.client.GetArticlesWithResponse(ctx, params, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get articles: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get articles", response.HTTPResponse, response.Body)
	}
	return response.JSON200, nil
}

func (c *generatedAPI) feed(ctx context.Context, offset, limit int, token string) (*api.MultipleArticlesResponse, error) {
	response, err := c.client.GetArticlesFeedWithResponse(ctx, &api.GetArticlesFeedParams{Offset: &offset, Limit: &limit}, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get feed", response.HTTPResponse, response.Body)
	}
	return response.JSON200, nil
}

func (c *generatedAPI) tags(ctx context.Context) ([]string, error) {
	response, err := c.client.GetTagsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get tags", response.HTTPResponse, response.Body)
	}
	return response.JSON200.Tags, nil
}

func (c *generatedAPI) article(ctx context.Context, slug, token string) (*api.Article, error) {
	response, err := c.client.GetArticleWithResponse(ctx, slug, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get article: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get article", response.HTTPResponse, response.Body)
	}
	return &response.JSON200.Article, nil
}

func (c *generatedAPI) comments(ctx context.Context, slug, token string) ([]api.Comment, error) {
	response, err := c.client.GetArticleCommentsWithResponse(ctx, slug, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get comments", response.HTTPResponse, response.Body)
	}
	return response.JSON200.Comments, nil
}

func (c *generatedAPI) profile(ctx context.Context, username, token string) (*api.Profile, error) {
	response, err := c.client.GetProfileByUsernameWithResponse(ctx, username, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get profile", response.HTTPResponse, response.Body)
	}
	return &response.JSON200.Profile, nil
}

func (c *generatedAPI) login(ctx context.Context, email, password string) (*authSession, error) {
	response, err := c.client.LoginWithResponse(ctx, api.LoginJSONRequestBody{User: api.LoginUser{Email: email, Password: password}})
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("login", response.HTTPResponse, response.Body)
	}
	return sessionFromResponse(&response.JSON200.User, response.HTTPResponse)
}

func (c *generatedAPI) register(ctx context.Context, username, email, password string) (*authSession, error) {
	response, err := c.client.CreateUserWithResponse(ctx, api.CreateUserJSONRequestBody{User: api.NewUser{Username: username, Email: email, Password: password}})
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	if response.JSON201 == nil {
		return nil, responseFailure("register", response.HTTPResponse, response.Body)
	}
	return sessionFromResponse(&response.JSON201.User, response.HTTPResponse)
}

func (c *generatedAPI) refresh(ctx context.Context, accessToken, refreshToken string) (*authSession, error) {
	if c.httpClient.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}
	request := c.refreshRequest.Clone(ctx)
	request.Header.Set("Authorization", "Token "+accessToken)
	request.AddCookie(&http.Cookie{
		Name: refreshTokenCookieName, Value: refreshToken, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	transport := c.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("refresh session: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, responseFailure("refresh session", response, body)
	}
	var payload api.UserResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	return sessionFromResponse(&payload.User, response)
}

func sessionFromResponse(user *api.User, response *http.Response) (*authSession, error) {
	if response != nil {
		for _, cookie := range response.Cookies() {
			if cookie.Name == refreshTokenCookieName && cookie.Value != "" {
				return &authSession{User: user, RefreshToken: cookie.Value}, nil
			}
		}
	}
	return nil, errors.New("authentication response has no refresh token")
}

func (c *generatedAPI) currentUser(ctx context.Context, token string) (*api.User, error) {
	response, err := c.client.GetCurrentUserWithResponse(ctx, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("get current user", response.HTTPResponse, response.Body)
	}
	return &response.JSON200.User, nil
}

func (c *generatedAPI) updateUser(ctx context.Context, token string, user api.UpdateUser) (*api.User, error) {
	response, err := c.client.UpdateCurrentUserWithResponse(ctx, api.UpdateCurrentUserJSONRequestBody{User: user}, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("update user", response.HTTPResponse, response.Body)
	}
	return &response.JSON200.User, nil
}

func (c *generatedAPI) createArticle(ctx context.Context, token string, article api.NewArticle) (*api.Article, error) {
	response, err := c.client.CreateArticleWithResponse(ctx, api.CreateArticleJSONRequestBody{Article: article}, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("create article: %w", err)
	}
	if response.JSON201 == nil {
		return nil, responseFailure("create article", response.HTTPResponse, response.Body)
	}
	return &response.JSON201.Article, nil
}

func (c *generatedAPI) updateArticle(ctx context.Context, slug, token string, article api.UpdateArticle) (*api.Article, error) {
	response, err := c.client.UpdateArticleWithResponse(ctx, slug, api.UpdateArticleJSONRequestBody{Article: article}, tokenEditors(token)...)
	if err != nil {
		return nil, fmt.Errorf("update article: %w", err)
	}
	if response.JSON200 == nil {
		return nil, responseFailure("update article", response.HTTPResponse, response.Body)
	}
	return &response.JSON200.Article, nil
}

func (c *generatedAPI) deleteArticle(ctx context.Context, slug, token string) error {
	response, err := c.client.DeleteArticleWithResponse(ctx, slug, tokenEditors(token)...)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}
	if response.HTTPResponse == nil || response.HTTPResponse.StatusCode != http.StatusNoContent {
		return responseFailure("delete article", response.HTTPResponse, response.Body)
	}
	return nil
}

func (c *generatedAPI) setFavorite(ctx context.Context, slug, token string, favorite bool) error {
	if favorite {
		response, err := c.client.CreateArticleFavoriteWithResponse(ctx, slug, tokenEditors(token)...)
		if err != nil {
			return fmt.Errorf("favorite article: %w", err)
		}
		if response.JSON200 == nil {
			return responseFailure("favorite article", response.HTTPResponse, response.Body)
		}
		return nil
	}
	response, err := c.client.DeleteArticleFavoriteWithResponse(ctx, slug, tokenEditors(token)...)
	if err != nil {
		return fmt.Errorf("unfavorite article: %w", err)
	}
	if response.JSON200 == nil {
		return responseFailure("unfavorite article", response.HTTPResponse, response.Body)
	}
	return nil
}

func (c *generatedAPI) setFollowing(ctx context.Context, username, token string, follow bool) error {
	if follow {
		response, err := c.client.FollowUserByUsernameWithResponse(ctx, username, tokenEditors(token)...)
		if err != nil {
			return fmt.Errorf("follow user: %w", err)
		}
		if response.JSON200 == nil {
			return responseFailure("follow user", response.HTTPResponse, response.Body)
		}
		return nil
	}
	response, err := c.client.UnfollowUserByUsernameWithResponse(ctx, username, tokenEditors(token)...)
	if err != nil {
		return fmt.Errorf("unfollow user: %w", err)
	}
	if response.JSON200 == nil {
		return responseFailure("unfollow user", response.HTTPResponse, response.Body)
	}
	return nil
}

func (c *generatedAPI) createComment(ctx context.Context, slug, body, token string) error {
	request := api.CreateArticleCommentJSONRequestBody{Comment: api.NewComment{Body: body}}
	response, err := c.client.CreateArticleCommentWithResponse(ctx, slug, request, tokenEditors(token)...)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	if response.JSON201 == nil {
		return responseFailure("create comment", response.HTTPResponse, response.Body)
	}
	return nil
}

func (c *generatedAPI) deleteComment(ctx context.Context, slug string, id int, token string) error {
	response, err := c.client.DeleteArticleCommentWithResponse(ctx, slug, id, tokenEditors(token)...)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if response.HTTPResponse == nil || response.HTTPResponse.StatusCode != http.StatusNoContent {
		return responseFailure("delete comment", response.HTTPResponse, response.Body)
	}
	return nil
}

func tokenEditors(token string) []api.RequestEditorFn {
	if token == "" {
		return nil
	}
	return []api.RequestEditorFn{func(_ context.Context, request *http.Request) error {
		request.Header.Set("Authorization", "Token "+token)
		return nil
	}}
}

func responseFailure(operation string, response *http.Response, body []byte) error {
	if response == nil {
		return fmt.Errorf("%s: response has no status", operation)
	}
	var model api.GenericErrorModel
	_ = json.Unmarshal(body, &model)
	if response.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	return &apiFailure{Status: response.StatusCode, Fields: model.Errors}
}
