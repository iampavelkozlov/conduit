import { apiClient } from './client'
import { ApiError } from './errors'
import type {
  Article,
  ArticleFilters,
  ArticlePage,
  Comment,
  NewArticle,
  Profile,
  UpdateArticle,
  UpdateUser,
  User,
} from './types'

function unwrap<T>(data: T | undefined, error: unknown, response: Response): T {
  if (data !== undefined) return data
  throw new ApiError(error, response.status)
}

export async function login(email: string, password: string): Promise<User> {
  const { data, error, response } = await apiClient.POST('/users/login', {
    body: { user: { email, password } },
  })
  return unwrap(data, error, response).user
}

export async function register(username: string, email: string, password: string): Promise<User> {
  const { data, error, response } = await apiClient.POST('/users', {
    body: { user: { username, email, password } },
  })
  return unwrap(data, error, response).user
}

export async function getCurrentUser(): Promise<User> {
  const { data, error, response } = await apiClient.GET('/user')
  return unwrap(data, error, response).user
}

export async function updateCurrentUser(user: UpdateUser): Promise<User> {
  const { data, error, response } = await apiClient.PUT('/user', { body: { user } })
  return unwrap(data, error, response).user
}

export async function getArticles(filters: ArticleFilters = {}): Promise<ArticlePage> {
  const { data, error, response } = await apiClient.GET('/articles', {
    params: { query: filters },
  })
  return unwrap(data, error, response)
}

export async function getFeed(offset = 0, limit = 10): Promise<ArticlePage> {
  const { data, error, response } = await apiClient.GET('/articles/feed', {
    params: { query: { offset, limit } },
  })
  return unwrap(data, error, response)
}

export async function getTags(): Promise<string[]> {
  const { data, error, response } = await apiClient.GET('/tags')
  return unwrap(data, error, response).tags
}

export async function getArticle(slug: string): Promise<Article> {
  const { data, error, response } = await apiClient.GET('/articles/{slug}', {
    params: { path: { slug } },
  })
  return unwrap(data, error, response).article
}

export async function createArticle(article: NewArticle): Promise<Article> {
  const { data, error, response } = await apiClient.POST('/articles', { body: { article } })
  return unwrap(data, error, response).article
}

export async function updateArticle(slug: string, article: UpdateArticle): Promise<Article> {
  const { data, error, response } = await apiClient.PUT('/articles/{slug}', {
    params: { path: { slug } },
    body: { article },
  })
  return unwrap(data, error, response).article
}

export async function deleteArticle(slug: string): Promise<void> {
  const { error, response } = await apiClient.DELETE('/articles/{slug}', {
    params: { path: { slug } },
  })
  if (!response.ok) throw new ApiError(error, response.status)
}

export async function getProfile(username: string): Promise<Profile> {
  const { data, error, response } = await apiClient.GET('/profiles/{username}', {
    params: { path: { username } },
  })
  return unwrap(data, error, response).profile
}

export async function setFollowing(username: string, following: boolean): Promise<Profile> {
  const request = following ? apiClient.POST : apiClient.DELETE
  const { data, error, response } = await request('/profiles/{username}/follow', {
    params: { path: { username } },
  })
  return unwrap(data, error, response).profile
}

export async function setFavorite(slug: string, favorited: boolean): Promise<Article> {
  const request = favorited ? apiClient.POST : apiClient.DELETE
  const { data, error, response } = await request('/articles/{slug}/favorite', {
    params: { path: { slug } },
  })
  return unwrap(data, error, response).article
}

export async function getComments(slug: string): Promise<Comment[]> {
  const { data, error, response } = await apiClient.GET('/articles/{slug}/comments', {
    params: { path: { slug } },
  })
  return unwrap(data, error, response).comments
}

export async function addComment(slug: string, body: string): Promise<Comment> {
  const { data, error, response } = await apiClient.POST('/articles/{slug}/comments', {
    params: { path: { slug } },
    body: { comment: { body } },
  })
  return unwrap(data, error, response).comment
}

export async function deleteComment(slug: string, id: number): Promise<void> {
  const { error, response } = await apiClient.DELETE('/articles/{slug}/comments/{id}', {
    params: { path: { slug, id } },
  })
  if (!response.ok) throw new ApiError(error, response.status)
}
