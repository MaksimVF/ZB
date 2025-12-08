



import React, { createContext, useState, useContext } from 'react'
import { admin } from '../api'

const AuthContext = createContext()

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  const login = async (username, password) => {
    try {
      const response = await admin.login(username, password)
      setUser(response.data.user)
      localStorage.setItem('adminToken', response.data.token)
      return true
    } catch (error) {
      console.error('Login failed:', error)
      return false
    }
  }

  const logout = () => {
    setUser(null)
    localStorage.removeItem('adminToken')
  }

  return (
    <AuthContext.Provider value={{ user, login, logout, loading }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)



