package frontend

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	api "conduit/internal/gen/http"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

//go:embed components/*.html
var templateAssets embed.FS

//go:embed static/styles.css
var stylesCSS []byte

type Options struct {
	CookieName   string
	CookieSecure bool
	CookieTTL    time.Duration
	MaxFormBytes int64
}

type App struct {
	api       conduitAPI
	logger    *slog.Logger
	templates map[string]*template.Template
	styles    http.Handler
	options   Options
}

type pageData struct {
	Title      string
	Page       string
	CurrentURL string
	User       *api.User
	Errors     []string
	Form       map[string]string
	Articles   any
	Tags       []string
	Article    *api.Article
	Comments   []api.Comment
	Profile    *api.Profile
	ActiveTag  string
	Feed       string
	Favorited  bool
	Editing    bool
	OwnArticle bool
	OwnProfile bool
	PageNumber int
	Pages      []int
}

func New(apiBaseURL string, httpClient *http.Client, logger *slog.Logger, options Options) (*App, error) {
	client, err := newGeneratedAPI(apiBaseURL, httpClient)
	if err != nil {
		return nil, err
	}
	return newApp(client, logger, options)
}

func newApp(client conduitAPI, logger *slog.Logger, options Options) (*App, error) {
	if client == nil {
		return nil, errors.New("API client must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if options.CookieName == "" || options.CookieTTL <= 0 || options.MaxFormBytes <= 0 {
		return nil, errors.New("frontend options are invalid")
	}
	templates, err := parseTemplates(templateAssets)
	if err != nil {
		return nil, err
	}
	return &App{api: client, logger: logger, templates: templates, styles: http.HandlerFunc(serveStyles), options: options}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /styles.css", a.styles)
	mux.HandleFunc("GET /", a.home)
	mux.HandleFunc("GET /article/{slug}", a.article)
	mux.HandleFunc("GET /profile/{username}", a.profile)
	mux.HandleFunc("GET /login", a.loginPage)
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("GET /register", a.registerPage)
	mux.HandleFunc("POST /register", a.register)
	mux.HandleFunc("POST /logout", a.logout)
	mux.HandleFunc("GET /settings", a.settingsPage)
	mux.HandleFunc("POST /settings", a.settings)
	mux.HandleFunc("GET /editor", a.editorPage)
	mux.HandleFunc("POST /editor", a.createArticle)
	mux.HandleFunc("GET /editor/{slug}", a.editArticlePage)
	mux.HandleFunc("POST /editor/{slug}", a.updateArticle)
	mux.HandleFunc("POST /article/{slug}/delete", a.deleteArticle)
	mux.HandleFunc("POST /article/{slug}/favorite", a.favoriteArticle)
	mux.HandleFunc("POST /profile/{username}/follow", a.followProfile)
	mux.HandleFunc("POST /article/{slug}/comments", a.createComment)
	mux.HandleFunc("POST /article/{slug}/comments/{id}/delete", a.deleteComment)
	return mux
}

func serveStyles(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(stylesCSS)
}

func (a *App) baseData(w http.ResponseWriter, r *http.Request, page, title string) (*pageData, bool) {
	data := &pageData{Title: title, Page: page, CurrentURL: requestURI(r), Form: map[string]string{}}
	token := a.sessionToken(r)
	if token == "" {
		return data, true
	}
	user, err := a.api.currentUser(r.Context(), token)
	if err == nil {
		data.User = user
		return data, true
	}
	if failureStatus(err) == http.StatusUnauthorized {
		a.clearSession(w)
		return data, true
	}
	a.serverError(w, r, err)
	return nil, false
}

func (a *App) requireUser(w http.ResponseWriter, r *http.Request) (string, *api.User, bool) {
	token := a.sessionToken(r)
	if token == "" {
		a.redirectLogin(w)
		return "", nil, false
	}
	user, err := a.api.currentUser(r.Context(), token)
	if err != nil {
		if failureStatus(err) == http.StatusUnauthorized {
			a.clearSession(w)
			a.redirectLogin(w)
			return "", nil, false
		}
		a.serverError(w, r, err)
		return "", nil, false
	}
	return token, user, true
}

func (a *App) parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, a.options.MaxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form.", http.StatusBadRequest)
		return false
	}
	return true
}

func (a *App) setSession(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name: a.options.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int(a.options.CookieTTL.Seconds()), Expires: time.Now().Add(a.options.CookieTTL),
	}
	cookie.Secure = a.options.CookieSecure
	http.SetCookie(w, cookie)
}

func (a *App) clearSession(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name: a.options.CookieName, Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0),
	}
	cookie.Secure = a.options.CookieSecure
	http.SetCookie(w, cookie)
}

func (a *App) sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(a.options.CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (a *App) render(w http.ResponseWriter, r *http.Request, name string, data *pageData) {
	tmpl, ok := a.templates[name]
	if !ok {
		a.serverError(w, r, fmt.Errorf("template %q is not registered", name))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		a.logger.ErrorContext(r.Context(), "render frontend template", "template", name, "error", err)
	}
}

func (a *App) mutationError(w http.ResponseWriter, r *http.Request, err error) {
	if failureStatus(err) == http.StatusUnauthorized {
		a.clearSession(w)
		a.redirectLogin(w)
		return
	}
	a.apiError(w, r, err)
}

func (a *App) apiError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNotFound) {
		http.NotFound(w, r)
		return
	}
	status := failureStatus(err)
	if status == http.StatusForbidden {
		http.Error(w, "Forbidden", status)
		return
	}
	a.serverError(w, r, err)
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.ErrorContext(r.Context(), "frontend request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	http.Error(w, "The Conduit API is temporarily unavailable.", http.StatusBadGateway)
}

func (a *App) redirectLogin(w http.ResponseWriter) {
	redirect(w, "/login")
}

func (a *App) redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	target := r.FormValue("return_to")
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		target = fallback
	}
	redirect(w, target)
}

func redirect(w http.ResponseWriter, target string) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		target = "/"
	}
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusSeeOther)
}

func parseTemplates(source fs.FS) (map[string]*template.Template, error) {
	functions := template.FuncMap{
		"date": func(value time.Time) string { return value.Format("January 2, 2006") },
		"image": func(value *string) string {
			if value == nil || *value == "" {
				return "https://static.productionready.io/images/smiley-cyrus.jpg"
			}
			return *value
		},
	}
	result := make(map[string]*template.Template)
	for _, name := range []string{"home", "article", "profile", "login", "register", "editor", "settings"} {
		files := []string{"components/layout.html", "components/partials.html", "components/" + name + ".html"}
		tmpl, err := template.New(name).Funcs(functions).ParseFS(source, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s template: %w", name, err)
		}
		result[name] = tmpl
	}
	return result, nil
}

func failureStatus(err error) int {
	failure, ok := errors.AsType[*apiFailure](err)
	if !ok {
		return 0
	}
	return failure.Status
}

func formErrors(err error) []string {
	failure, ok := errors.AsType[*apiFailure](err)
	if !ok || len(failure.Fields) == 0 {
		return []string{"Unable to complete the request"}
	}
	var result []string
	for _, field := range slices.Sorted(maps.Keys(failure.Fields)) {
		for _, message := range failure.Fields[field] {
			result = append(result, field+" "+message)
		}
	}
	return result
}

func pageNumber(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 {
		return 1
	}
	return number
}

func pagination(total, pageSize int) []int {
	count := (total + pageSize - 1) / pageSize
	pages := make([]int, count)
	for index := range count {
		pages[index] = index + 1
	}
	return pages
}

func requestURI(r *http.Request) string {
	if r.URL.RequestURI() == "" {
		return "/"
	}
	return r.URL.RequestURI()
}
