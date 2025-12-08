












import { useState } from 'react'
import { auth } from '../api'
import { useNotification } from '../context/NotificationContext'

export default function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { addNotification } = useNotification()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setMessage('')

    if (!email) {
      setError('Please enter your email address')
      addNotification('Please enter your email address', 'error')
      return
    }

    setLoading(true)
    try {
      await auth.forgotPassword({ email })
      setMessage('Password reset link sent to your email')
      addNotification('Password reset link sent! Check your email.', 'success')
    } catch (err) {
      const errorMessage = err.response?.data?.message || 'Failed to send reset link'
      setError(errorMessage)
      addNotification('Failed to send reset link: ' + errorMessage, 'error')
      console.error('Forgot password error:', errorMessage)
    }
    setLoading(false)
  }

  return (
    <div className="flex justify-center items-center h-screen bg-gray-100">
      <div className="bg-white p-12 rounded-3xl shadow-2xl w-full max-w-md" role="main" aria-labelledby="forgot-password-title">
        <h1 id="forgot-password-title" className="text-4xl font-bold mb-8 text-center text-gray-800">Forgot Password</h1>

        {error && (
          <div
            role="alert"
            aria-live="assertive"
            className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg"
          >
            {error}
          </div>
        )}
        {message && (
          <div
            role="status"
            aria-live="polite"
            className="mb-6 p-4 bg-green-100 text-green-700 rounded-lg"
          >
            {message}
          </div>
        )}

        <form onSubmit={handleSubmit} aria-labelledby="forgot-password-form-title">
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

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-indigo-600 text-white py-4 rounded-xl hover:bg-indigo-700 transition text-xl font-bold disabled:opacity-50"
            aria-busy={loading}
          >
            {loading ? 'Sending...' : 'Send Reset Link'}
          </button>
        </form>

        <div className="mt-6 text-center">
          <p className="text-gray-600">
            Remember your password?{' '}
            <a
              href="/login"
              className="text-indigo-600 hover:underline"
              aria-label="Go to login page"
            >
              Login
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}














