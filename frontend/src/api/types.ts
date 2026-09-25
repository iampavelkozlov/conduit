import type { components, paths } from './generated/schema'

export type User = components['schemas']['User']
export type Profile = components['schemas']['Profile']
export type Article = components['schemas']['Article']
export type Comment = components['schemas']['Comment']
export type NewArticle = components['schemas']['NewArticle']
export type UpdateArticle = components['schemas']['UpdateArticle']
export type UpdateUser = components['schemas']['UpdateUser']
export type ArticleSummary = paths['/articles']['get']['responses']['200']['content']['application/json']['articles'][number]

export interface ArticlePage {
  articles: ArticleSummary[]
  articlesCount: number
}

export interface ArticleFilters {
  tag?: string
  author?: string
  favorited?: string
  offset?: number
  limit?: number
}
