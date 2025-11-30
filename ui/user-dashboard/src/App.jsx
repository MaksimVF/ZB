





import { BrowserRouter as Router, Routes, Route, Suspense, lazy } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import PrivateRoute from './components/PrivateRoute'
import ErrorBoundary from './components/ErrorBoundary'
import { useState, useEffect } from 'react'

// Lazy load pages
const Dashboard = lazy(() => import('./pages/Dashboard'))
const TopUp = lazy(() => import('./pages/TopUp'))
const Usage = lazy(() => import('./pages/Usage'))
const Settings = lazy(() => import('./pages/Settings'))
const History = lazy(() => import('./pages/History'))
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
            </Routes>
          </Router>
        </AuthProvider>
      </ErrorBoundary>
    </div>
  )
}

export default App






