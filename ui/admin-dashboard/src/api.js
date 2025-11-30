


import axios from 'axios'
import { setupCache } from 'axios-cache-adapter'

const cache = setupCache({
  maxAge: 15 * 60 * 1000, // 15 minutes cache
  exclude: { query: false }
})

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'https://api.yourdomain.com',
  headers: {
    'X-Admin-Key': import.meta.env.VITE_ADMIN_KEY || 'superadmin2025'
  },
  adapter: cache.adapter
})

const handleApiError = (error) => {
  if (error.response) {
    // Server responded with status other than 2xx
    console.error('API Error:', error.response.data)
    throw new Error(error.response.data.message || 'API Error')
  } else if (error.request) {
    // No response received
    console.error('Network Error:', error.request)
    throw new Error('Network Error')
  } else {
    // Something else happened
    console.error('Error:', error.message)
    throw new Error(error.message)
  }
}

export const admin = {
  stats: async () => {
    try {
      const response = await api.get('/admin/stats')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  users: async () => {
    try {
      const response = await api.get('/admin/users')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  adjustBalance: async (userId, amount, reason) => {
    try {
      const response = await api.post('/admin/adjust', { user_id: userId, amount_usd: amount, reason })
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  pricing: async () => {
    try {
      const response = await api.get('/admin/pricing')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  savePricing: async (data) => {
    try {
      const response = await api.post('/admin/pricing', data)
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  secrets: async () => {
    try {
      const response = await api.get('/admin/api/secrets')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  saveSecret: async (name, value) => {
    try {
      const response = await api.post('/admin/api/secrets', { name, value })
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  rateLimits: async () => {
    try {
      const response = await api.get('/admin/api/rate-limits')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  logs: async (page = 1) => {
    try {
      const response = await api.get(`/admin/logs?page=${page}`)
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  revenue: async () => {
    try {
      const response = await api.get('/admin/revenue')
      return response
    } catch (error) {
      handleApiError(error)
    }
  },
  login: async (username, password) => {
    try {
      const response = await api.post('/admin/login', { username, password })
      return response
    } catch (error) {
      handleApiError(error)
    }
  }
}


