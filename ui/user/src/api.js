import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'https://api.yourdomain.com',
  headers: {
    'Authorization': `Bearer ${localStorage.getItem('token') || ''}`
  }
})

export const auth = {
  login: (apiKey) => api.post('/auth/login', { api_key: apiKey }),
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
