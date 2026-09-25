import createClient, { type Middleware } from 'openapi-fetch'

import type { paths } from './generated/schema'
import { getAccessToken, setAccessToken } from './session'
import type { User } from './types'

const retryRequests = new Map<string, Request>()
let refreshPromise: Promise<string> | null = null

function refreshAccessToken() {
  if (refreshPromise) return refreshPromise

  refreshPromise = fetch('/internal/auth/refresh', {
    method: 'POST',
    credentials: 'include',
    headers: { Authorization: `Token ${getAccessToken()}` },
  })
    .then(async (response) => {
      if (!response.ok) return ''
      const payload = (await response.json()) as { user: User }
      return payload.user.token
    })
    .catch(() => '')
    .finally(() => {
      refreshPromise = null
    })

  return refreshPromise
}

const authMiddleware: Middleware = {
  onRequest({ id, request }) {
    const token = getAccessToken()
    if (token) request.headers.set('Authorization', `Token ${token}`)

    retryRequests.set(id, request.clone())
    return request
  },

  async onResponse({ id, response, schemaPath }) {
    const retryRequest = retryRequests.get(id)
    retryRequests.delete(id)

    if (
      response.status !== 401 ||
      !retryRequest ||
      !getAccessToken() ||
      schemaPath === '/users/login' ||
      schemaPath === '/users'
    ) {
      return response
    }

    const token = await refreshAccessToken()
    if (!token) {
      setAccessToken('')
      return response
    }

    setAccessToken(token)

    const headers = new Headers(retryRequest.headers)
    headers.set('Authorization', `Token ${token}`)
    return fetch(new Request(retryRequest, { headers }))
  },

  onError({ id }) {
    retryRequests.delete(id)
  },
}

export const apiClient = createClient<paths>({
  baseUrl: '/api',
  credentials: 'include',
})

apiClient.use(authMiddleware)
