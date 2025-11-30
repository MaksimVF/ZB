





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
  me: () => api.get('/user/me')
}

export const billing = {
  balance: () => api.get('/billing/balance'),
  topup: (amount) => api.post('/billing/create-checkout', { amount_usd: amount }),
  history: () => api.get('/billing/history'),
  usage: () => api.get('/billing/usage')
}

export const settings = {
  get: () => api.get('/user/settings'),
  save: (data) => api.post('/user/settings', data)
}

export const apiKeys = {
  getApiKeys: () => api.get('/user/api-keys'),
  createApiKey: (data) => api.post('/user/api-keys', data),
  deleteApiKey: (id) => api.delete(`/user/api-keys/${id}`)
}







