import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const ArticleView = () => import('../views/ArticleView/ArticleView.vue')
const EditorView = () => import('../views/EditorView/EditorView.vue')
const HomeView = () => import('../views/HomeView/HomeView.vue')
const LoginView = () => import('../views/LoginView/LoginView.vue')
const NotFoundView = () => import('../views/NotFoundView/NotFoundView.vue')
const ProfileView = () => import('../views/ProfileView/ProfileView.vue')
const RegisterView = () => import('../views/RegisterView/RegisterView.vue')
const SettingsView = () => import('../views/SettingsView/SettingsView.vue')

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
