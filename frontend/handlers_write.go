package frontend

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	api "conduit/internal/gen/http"
)

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !a.parseForm(w, r) {
		return
	}
	form := map[string]string{"email": r.FormValue("email")}
	session, err := a.api.login(r.Context(), form["email"], r.FormValue("password"))
	if err != nil {
		a.render(w, r, "login", &pageData{Title: "Sign in — Conduit", Page: "login", Errors: formErrors(err), Form: form})
		return
	}
	a.setSession(w, session)
	redirect(w, "/")
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	if !a.parseForm(w, r) {
		return
	}
	form := map[string]string{"username": r.FormValue("username"), "email": r.FormValue("email")}
	session, err := a.api.register(r.Context(), form["username"], form["email"], r.FormValue("password"))
	if err != nil {
		a.render(w, r, "register", &pageData{Title: "Sign up — Conduit", Page: "register", Errors: formErrors(err), Form: form})
		return
	}
	a.setSession(w, session)
	redirect(w, "/")
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	a.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	token, user, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	form := map[string]string{
		"image": r.FormValue("image"), "username": r.FormValue("username"),
		"bio": r.FormValue("bio"), "email": r.FormValue("email"),
	}
	update := api.UpdateUser{Image: new(form["image"]), Username: new(form["username"]), Bio: new(form["bio"]), Email: new(form["email"])}
	if password := r.FormValue("password"); password != "" {
		update.Password = &password
	}
	updated, err := a.api.updateUser(r.Context(), token, update)
	if err != nil {
		if failureStatus(err) == http.StatusUnauthorized {
			a.mutationError(w, r, err)
			return
		}
		a.render(w, r, "settings", &pageData{Title: "Your Settings — Conduit", Page: "settings", User: user, Errors: formErrors(err), Form: form})
		return
	}
	a.setCookie(w, a.options.CookieName, updated.Token)
	redirect(w, "/profile/"+url.PathEscape(updated.Username))
}

func (a *App) createArticle(w http.ResponseWriter, r *http.Request) {
	token, user, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	form := articleForm(r)
	tags := splitTags(form["tags"])
	article, err := a.api.createArticle(r.Context(), token, api.NewArticle{
		Title: form["title"], Description: form["description"], Body: form["body"], TagList: &tags,
	})
	if err != nil {
		a.render(w, r, "editor", &pageData{Title: "New Article — Conduit", Page: "editor", User: user, Errors: formErrors(err), Form: form})
		return
	}
	redirect(w, "/article/"+url.PathEscape(article.Slug))
}

func (a *App) updateArticle(w http.ResponseWriter, r *http.Request) {
	token, user, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	slug := r.PathValue("slug")
	form := articleForm(r)
	tags := splitTags(form["tags"])
	article, err := a.api.updateArticle(r.Context(), slug, token, api.UpdateArticle{
		Title: new(form["title"]), Description: new(form["description"]), Body: new(form["body"]), TagList: &tags,
	})
	if err != nil {
		if failureStatus(err) == http.StatusUnauthorized {
			a.mutationError(w, r, err)
			return
		}
		a.render(w, r, "editor", &pageData{Title: "Edit Article — Conduit", Page: "editor", User: user, Editing: true, Article: &api.Article{Slug: slug}, Errors: formErrors(err), Form: form})
		return
	}
	redirect(w, "/article/"+url.PathEscape(article.Slug))
}

func (a *App) deleteArticle(w http.ResponseWriter, r *http.Request) {
	token, _, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if err := a.api.deleteArticle(r.Context(), r.PathValue("slug"), token); err != nil {
		a.mutationError(w, r, err)
		return
	}
	redirect(w, "/")
}

func (a *App) favoriteArticle(w http.ResponseWriter, r *http.Request) {
	token, _, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	slug := r.PathValue("slug")
	if err := a.api.setFavorite(r.Context(), slug, token, r.FormValue("favorite") == "true"); err != nil {
		a.mutationError(w, r, err)
		return
	}
	a.redirectBack(w, r, "/article/"+slug)
}

func (a *App) followProfile(w http.ResponseWriter, r *http.Request) {
	token, _, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	username := r.PathValue("username")
	if err := a.api.setFollowing(r.Context(), username, token, r.FormValue("follow") == "true"); err != nil {
		a.mutationError(w, r, err)
		return
	}
	a.redirectBack(w, r, "/profile/"+username)
}

func (a *App) createComment(w http.ResponseWriter, r *http.Request) {
	token, _, ok := a.requireUser(w, r)
	if !ok || !a.parseForm(w, r) {
		return
	}
	slug := r.PathValue("slug")
	if err := a.api.createComment(r.Context(), slug, r.FormValue("body"), token); err != nil {
		a.mutationError(w, r, err)
		return
	}
	redirect(w, "/article/"+url.PathEscape(slug))
}

func (a *App) deleteComment(w http.ResponseWriter, r *http.Request) {
	token, _, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	slug := r.PathValue("slug")
	if err := a.api.deleteComment(r.Context(), slug, id, token); err != nil {
		a.mutationError(w, r, err)
		return
	}
	redirect(w, "/article/"+url.PathEscape(slug))
}

func articleForm(r *http.Request) map[string]string {
	return map[string]string{
		"title": r.FormValue("title"), "description": r.FormValue("description"),
		"body": r.FormValue("body"), "tags": r.FormValue("tags"),
	}
}

func splitTags(value string) []string {
	seen := make(map[string]struct{})
	var tags []string
	for tag := range strings.FieldsFuncSeq(value, func(r rune) bool { return r == ',' }) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func joinTags(tags []string) string { return strings.Join(tags, ", ") }
