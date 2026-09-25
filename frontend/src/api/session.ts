const tokenKey = 'conduit_access_token'

let accessToken = localStorage.getItem(tokenKey) ?? ''

export function getAccessToken() {
  return accessToken
}

export function setAccessToken(token: string) {
  accessToken = token

  if (token) {
    localStorage.setItem(tokenKey, token)
  } else {
    localStorage.removeItem(tokenKey)
  }
}
