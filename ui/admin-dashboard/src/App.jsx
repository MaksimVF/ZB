


import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Users from './pages/Users'
import Pricing from './pages/Pricing'
import Secrets from './pages/Secrets'
import RateLimits from './pages/RateLimits'
import Logs from './pages/Logs'
import Revenue from './pages/Revenue'
import Login from './pages/Login'
import { AuthProvider } from './context/AuthContext'
import PrivateRoute from './components/PrivateRoute'

function App() {
  return (
    <AuthProvider>
      <Router>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/" element={<PrivateRoute><Dashboard /></PrivateRoute>} />
          <Route path="/admin" element={<PrivateRoute><Dashboard /></PrivateRoute>} />
          <Route path="/admin/users" element={<PrivateRoute><Users /></PrivateRoute>} />
          <Route path="/admin/pricing" element={<PrivateRoute><Pricing /></PrivateRoute>} />
          <Route path="/admin/secrets" element={<PrivateRoute><Secrets /></PrivateRoute>} />
          <Route path="/admin/rate-limits" element={<PrivateRoute><RateLimits /></PrivateRoute>} />
          <Route path="/admin/logs" element={<PrivateRoute><Logs /></PrivateRoute>} />
          <Route path="/admin/revenue" element={<PrivateRoute><Revenue /></PrivateRoute>} />
        </Routes>
      </Router>
    </AuthProvider>
  )
}

export default App


