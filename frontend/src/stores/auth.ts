import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import * as conduitApi from '../api/conduit'
import { getAccessToken, setAccessToken } from '../api/session'
import type { UpdateUser, User } from '../api/types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = computed(() => user.value !== null)

  async function restoreSession() {
    if (!getAccessToken()) return

    try {
      user.value = await conduitApi.getCurrentUser()
      setAccessToken(user.value.token)
    } catch {
      logout()
    }
  }

  async function login(email: string, password: string) {
    logout()
    user.value = await conduitApi.login(email, password)
    setAccessToken(user.value.token)
  }

  async function register(username: string, email: string, password: string) {
    logout()
    user.value = await conduitApi.register(username, email, password)
    setAccessToken(user.value.token)
  }

  async function update(details: UpdateUser) {
    user.value = await conduitApi.updateCurrentUser(details)
    setAccessToken(user.value.token)
  }

  function logout() {
    user.value = null
    setAccessToken('')
  }

  return { user, isAuthenticated, restoreSession, login, register, update, logout }
})
