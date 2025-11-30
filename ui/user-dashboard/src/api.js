





import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'https://api.вашдомен.com',
  headers: {
    'Authorization': `Bearer ${localStorage.getItem('token') || ''}`
  }
})

export const auth = {
  login: (email, password) => api.post('/auth/login', { email, password }),
  register: (data) => api.post('/auth/register', data),
  me: () => api.get('/user/me'),
  get2FAStatus: () => api.get('/auth/2fa/status'),
  enable2FA: () => api.post('/auth/2fa/enable'),
  verify2FA: (data) => api.post('/auth/2fa/verify', data),
  disable2FA: () => api.post('/auth/2fa/disable'),
  forgotPassword: (data) => api.post('/auth/forgot-password', data),
  resetPassword: (token, data) => api.post(`/auth/reset-password/${token}`, data)
}

export const billing = {
  balance: () => api.get('/billing/balance'),
  topup: (amount, options) => api.post('/billing/create-checkout', { amount_usd: amount, ...options }),
  history: () => api.get('/billing/history'),
  usage: () => api.get('/billing/usage'),
  getSubscriptionPlans: () => api.get('/billing/subscription-plans'),
  subscribe: (planId) => api.post('/billing/subscribe', { plan_id: planId })
}

export const settings = {
  get: () => api.get('/user/settings'),
  save: (data) => api.post('/user/settings', data)
}

export const apiKeys = {
  getApiKeys: () => api.get('/user/api-keys'),
  createApiKey: (data) => api.post('/user/api-keys', data),
  deleteApiKey: (id) => api.delete(`/user/api-keys/${id}`),
  rotateApiKey: (id) => api.post(`/user/api-keys/${id}/rotate`),
  getApiKeyUsage: (id) => api.get(`/user/api-keys/${id}/usage`)
}







