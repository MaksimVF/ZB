


import { BrowserRouter as Router, Routes, Route, Suspense, lazy } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import PrivateRoute from './components/PrivateRoute'
import ErrorBoundary from './components/ErrorBoundary'
import { useState, useEffect } from 'react'

// Lazy load pages
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Users = lazy(() => import('./pages/Users'))
const Pricing = lazy(() => import('./pages/Pricing'))
const Secrets = lazy(() => import('./pages/Secrets'))
const RateLimits = lazy(() => import('./pages/RateLimits'))
const Logs = lazy(() => import('./pages/Logs'))
const Revenue = lazy(() => import('./pages/Revenue'))
const Login = lazy(() => import('./pages/Login'))

function App() {
  const [darkMode, setDarkMode] = useState(false)

  useEffect(() => {
    if (darkMode) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }, [darkMode])

  return (
    <div className={darkMode ? 'dark' : ''}>
      <button
        onClick={() => setDarkMode(!darkMode)}
        className="fixed top-4 right-4 p-2 bg-gray-200 dark:bg-gray-700 rounded z-50"
        aria-label="Toggle dark mode"
      >
        {darkMode ? '🌙' : '☀️'}
      </button>

      <ErrorBoundary>
        <AuthProvider>
          <Router>
            <Routes>
              <Route path="/login" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Login />
                </Suspense>
              } />
              <Route path="/" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Dashboard /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Dashboard /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/users" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Users /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/pricing" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Pricing /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/secrets" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Secrets /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/rate-limits" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><RateLimits /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/logs" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Logs /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/admin/revenue" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Revenue /></PrivateRoute>
                </Suspense>
              } />
            </Routes>
          </Router>
        </AuthProvider>
      </ErrorBoundary>
    </div>
  )
}

export default App


