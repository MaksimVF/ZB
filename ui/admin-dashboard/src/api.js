


import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'https://api.yourdomain.com',
  headers: {
    'X-Admin-Key': import.meta.env.VITE_ADMIN_KEY || 'superadmin2025'
  }
})

export const admin = {
  stats: () => api.get('/admin/stats'),
  users: () => api.get('/admin/users'),
  adjustBalance: (userId, amount, reason) => api.post('/admin/adjust', { user_id: userId, amount_usd: amount, reason }),
  pricing: () => api.get('/admin/pricing'),
  savePricing: (data) => api.post('/admin/pricing', data),
  secrets: () => api.get('/admin/api/secrets'),
  saveSecret: (name, value) => api.post('/admin/api/secrets', { name, value }),
  rateLimits: () => api.get('/admin/api/rate-limits'),
  logs: (page = 1) => api.get(`/admin/logs?page=${page}`),
  revenue: () => api.get('/admin/revenue'),
  login: (username, password) => api.post('/admin/login', { username, password })
}


