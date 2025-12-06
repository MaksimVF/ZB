









import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login } = useAuth()
  const navigate = useNavigate()
  const { addNotification } = useNotification()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (!email || !password) {
      setError('Please enter both email and password')
      addNotification('Please enter both email and password', 'error')
      return
    }

    setLoading(true)
    try {
      const success = await login(email, password)
      if (success) {
        addNotification('Login successful! Welcome back.', 'success')
        navigate('/dashboard')
      } else {
        setError('Invalid email or password')
        addNotification('Login failed. Please check your credentials.', 'error')
      }
    } catch (err) {
      setError('An unexpected error occurred. Please try again.')
      addNotification('Login error: ' + (err.message || 'Unknown error'), 'error')
      console.error('Login error:', err)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex justify-center items-center h-screen bg-gray-100">
      <div className="bg-white p-12 rounded-3xl shadow-2xl w-full max-w-md" role="main" aria-labelledby="login-title">
        <h1 id="login-title" className="text-4xl font-bold mb-8 text-center text-gray-800">User Login</h1>

        {error && (
          <div
            role="alert"
            aria-live="assertive"
            className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg"
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} aria-labelledby="login-form-title">
          <div className="mb-6">
            <label
              htmlFor="email"
              className="block text-gray-700 mb-2"
            >
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full p-4 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500"
              required
              aria-invalid={!email}
              aria-describedby={!email ? 'email-error' : undefined}
            />
            {!email && (
              <p
                id="email-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                Email is required
              </p>
            )}
          </div>

          <div className="mb-6">
            <label
              htmlFor="password"
              className="block text-gray-700 mb-2"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full p-4 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500"
              required
              aria-invalid={!password}
              aria-describedby={!password ? 'password-error' : undefined}
            />
            {!password && (
              <p
                id="password-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                Password is required
              </p>
            )}
          </div>

          <button
            type="submit"
            className="w-full bg-indigo-600 text-white py-4 rounded-xl hover:bg-indigo-700 transition text-xl font-bold"
            disabled={loading}
            aria-busy={loading}
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>

        <div className="mt-6 text-center">
          <p className="text-gray-600">
            Don't have an account?{' '}
            <a
              href="/register"
              className="text-indigo-600 hover:underline"
              aria-label="Go to registration page"
            >
              Register
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}











