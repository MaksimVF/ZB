









import { createContext, useContext, useState, useEffect } from 'react'
import { auth } from '../api'
import { useNavigate } from 'react-router-dom'

const AuthContext = createContext()

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [language, setLanguage] = useState('en')
  const [currency, setCurrency] = useState('USD')
  const navigate = useNavigate()

  useEffect(() => {
    const checkAuth = async () => {
      const token = localStorage.getItem('token')
      if (token) {
        try {
          const res = await auth.me()
          setUser(res.data)
          // Load user preferences
          const prefs = await auth.getPreferences()
          setLanguage(prefs.data.language || 'en')
          setCurrency(prefs.data.currency || 'USD')
        } catch (e) {
          localStorage.removeItem('token')
        }
      }
      setLoading(false)
    }
    checkAuth()
  }, [])

  const login = async (email, password) => {
    try {
      const res = await auth.login(email, password)
      localStorage.setItem('token', res.data.token)
      setUser(res.data.user)
      return true
    } catch (e) {
      return false
    }
  }

  const logout = () => {
    localStorage.removeItem('token')
    setUser(null)
    navigate('/login')
  }

  const updatePreferences = async (preferences) => {
    try {
      await auth.savePreferences(preferences)
      setLanguage(preferences.language || language)
      setCurrency(preferences.currency || currency)
    } catch (e) {
      console.error('Failed to save preferences')
    }
  }

  return (
    <AuthContext.Provider value={{ user, login, logout, loading, language, currency, updatePreferences }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)










