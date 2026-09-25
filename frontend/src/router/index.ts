import { createRouter, createWebHistory } from 'vue-router'

import ArticleView from '../views/ArticleView/ArticleView.vue'
import EditorView from '../views/EditorView/EditorView.vue'
import HomeView from '../views/HomeView/HomeView.vue'
import LoginView from '../views/LoginView/LoginView.vue'
import NotFoundView from '../views/NotFoundView/NotFoundView.vue'
import ProfileView from '../views/ProfileView/ProfileView.vue'
import RegisterView from '../views/RegisterView/RegisterView.vue'
import SettingsView from '../views/SettingsView/SettingsView.vue'
import { useAuthStore } from '../stores/auth'

export const router = createRouter({
  history: createWebHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/login', name: 'login', component: LoginView, meta: { guestOnly: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { guestOnly: true } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { requiresAuth: true } },
    { path: '/editor/:slug?', name: 'editor', component: EditorView, meta: { requiresAuth: true } },
    { path: '/article/:slug', name: 'article', component: ArticleView },
    { path: '/profile/:username', name: 'profile', component: ProfileView },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.isAuthenticated) return { name: 'home' }
})
