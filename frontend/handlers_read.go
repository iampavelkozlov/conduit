package frontend

import (
	"net/http"

	api "conduit/internal/gen/http"
)

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, ok := a.baseData(w, r, "home", "Conduit")
	if !ok {
		return
	}
	page := pageNumber(r.URL.Query().Get("page"))
	data.PageNumber = page
	data.ActiveTag = r.URL.Query().Get("tag")
	data.Feed = r.URL.Query().Get("feed")
	token := a.sessionToken(r)

	var (
		articles *api.MultipleArticlesResponse
		err      error
	)
	if data.Feed == "following" {
		if data.User == nil {
			a.redirectLogin(w)
			return
		}
		articles, err = a.api.feed(r.Context(), (page-1)*defaultPageSize, defaultPageSize, token)
	} else {
		query := articleQuery{Offset: (page - 1) * defaultPageSize, Limit: defaultPageSize}
		if data.ActiveTag != "" {
			query.Tag = &data.ActiveTag
		}
		articles, err = a.api.articles(r.Context(), query, token)
	}
	if err != nil {
		a.apiError(w, r, err)
		return
	}
	data.Articles = articles.Articles
	data.Pages = pagination(articles.ArticlesCount, defaultPageSize)
	data.Tags, err = a.api.tags(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, "home", data)
}

func (a *App) article(w http.ResponseWriter, r *http.Request) {
	data, ok := a.baseData(w, r, "article", "Article — Conduit")
	if !ok {
		return
	}
	slug := r.PathValue("slug")
	token := a.sessionToken(r)
	article, err := a.api.article(r.Context(), slug, token)
	if err != nil {
		a.apiError(w, r, err)
		return
	}
	comments, err := a.api.comments(r.Context(), slug, token)
	if err != nil {
		a.apiError(w, r, err)
		return
	}
	data.Title = article.Title + " — Conduit"
	data.Article = article
	data.Comments = comments
	data.OwnArticle = data.User != nil && data.User.Username == article.Author.Username
	a.render(w, r, "article", data)
}

func (a *App) profile(w http.ResponseWriter, r *http.Request) {
	data, ok := a.baseData(w, r, "profile", "Profile — Conduit")
	if !ok {
		return
	}
	username := r.PathValue("username")
	token := a.sessionToken(r)
	profile, err := a.api.profile(r.Context(), username, token)
	if err != nil {
		a.apiError(w, r, err)
		return
	}
	data.Favorited = r.URL.Query().Get("tab") == "favorites"
	query := articleQuery{Offset: 0, Limit: maxPageSize}
	if data.Favorited {
		query.Favorited = &username
	} else {
		query.Author = &username
	}
	articles, err := a.api.articles(r.Context(), query, token)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	data.Title = profile.Username + " — Conduit"
	data.Profile = profile
	data.Articles = articles.Articles
	data.OwnProfile = data.User != nil && data.User.Username == profile.Username
	a.render(w, r, "profile", data)
}

func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	a.authPage(w, r, "login", "Sign in — Conduit")
}

func (a *App) registerPage(w http.ResponseWriter, r *http.Request) {
	a.authPage(w, r, "register", "Sign up — Conduit")
}

func (a *App) authPage(w http.ResponseWriter, r *http.Request, page, title string) {
	data, ok := a.baseData(w, r, page, title)
	if !ok {
		return
	}
	if data.User != nil {
		redirect(w, "/")
		return
	}
	a.render(w, r, page, data)
}

func (a *App) settingsPage(w http.ResponseWriter, r *http.Request) {
	_, user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	data := &pageData{Title: "Your Settings — Conduit", Page: "settings", CurrentURL: requestURI(r), User: user, Form: userForm(user)}
	a.render(w, r, "settings", data)
}

func (a *App) editorPage(w http.ResponseWriter, r *http.Request) {
	_, user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	a.render(w, r, "editor", &pageData{Title: "New Article — Conduit", Page: "editor", User: user, Form: map[string]string{}})
}

func (a *App) editArticlePage(w http.ResponseWriter, r *http.Request) {
	token, user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	article, err := a.api.article(r.Context(), r.PathValue("slug"), token)
	if err != nil {
		a.apiError(w, r, err)
		return
	}
	if article.Author.Username != user.Username {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	data := &pageData{
		Title: "Edit Article — Conduit", Page: "editor", User: user, Article: article, Editing: true,
		Form: map[string]string{"title": article.Title, "description": article.Description, "body": article.Body, "tags": joinTags(article.TagList)},
	}
	a.render(w, r, "editor", data)
}

func userForm(user *api.User) map[string]string {
	result := map[string]string{"username": user.Username, "email": user.Email}
	if user.Bio != nil {
		result["bio"] = *user.Bio
	}
	if user.Image != nil {
		result["image"] = *user.Image
	}
	return result
}
