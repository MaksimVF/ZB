





import { BrowserRouter as Router, Routes, Route, Suspense, lazy } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import PrivateRoute from './components/PrivateRoute'
import ErrorBoundary from './components/ErrorBoundary'
import { useState, useEffect } from 'react'
import CurrencySelector from './components/CurrencySelector'

// Lazy load pages
const Dashboard = lazy(() => import('./pages/Dashboard'))
const TopUp = lazy(() => import('./pages/TopUp'))
const Usage = lazy(() => import('./pages/Usage'))
const Settings = lazy(() => import('./pages/Settings'))
const History = lazy(() => import('./pages/History'))
const ApiKeys = lazy(() => import('./pages/ApiKeys'))
const Security = lazy(() => import('./pages/Security'))
const HelpCenter = lazy(() => import('./pages/HelpCenter'))
const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'))

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

      <div className="fixed top-4 right-24 z-50">
        <CurrencySelector />
      </div>

      <ErrorBoundary>
        <AuthProvider>
          <Router>
            <Routes>
              <Route path="/login" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Login />
                </Suspense>
              } />
              <Route path="/register" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Register />
                </Suspense>
              } />
              <Route path="/forgot-password" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <ForgotPassword />
                </Suspense>
              } />
              <Route path="/" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Dashboard /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/dashboard" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Dashboard /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/topup" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><TopUp /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/usage" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Usage /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/settings" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Settings /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/history" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><History /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/api-keys" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><ApiKeys /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/security" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><Security /></PrivateRoute>
                </Suspense>
              } />
              <Route path="/help" element={
                <Suspense fallback={<div>Loading...</div>}>
                  <PrivateRoute><HelpCenter /></PrivateRoute>
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






