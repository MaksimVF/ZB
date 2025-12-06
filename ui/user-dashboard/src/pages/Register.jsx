










import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { auth } from '../api'
import PasswordStrength from '../components/PasswordStrength'
import { useNotification } from '../context/NotificationContext'

export default function Register() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [emailError, setEmailError] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const navigate = useNavigate()
  const { addNotification } = useNotification()

  const validateEmail = (email) => {
    const re = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
    return re.test(email)
  }

  const validatePassword = (password) => {
    if (password.length < 10) return 'Password must be at least 10 characters'
    if (!/[A-Z]/.test(password)) return 'Password must contain at least one uppercase letter'
    if (!/[0-9]/.test(password)) return 'Password must contain at least one number'
    if (!/[^A-Za-z0-9]/.test(password)) return 'Password must contain at least one special character'
    if (password.length > 50) return 'Password must be less than 50 characters'
    return ''
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    // Validate email
    if (!validateEmail(email)) {
      setEmailError('Please enter a valid email address')
      addNotification('Please enter a valid email address', 'error')
      return
    } else {
      setEmailError('')
    }

    // Validate password
    const passwordValidation = validatePassword(password)
    if (passwordValidation) {
      setPasswordError(passwordValidation)
      addNotification(passwordValidation, 'error')
      return
    } else {
      setPasswordError('')
    }

    // Check password match
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      addNotification('Passwords do not match', 'error')
      return
    }

    setLoading(true)
    try {
      await auth.register({ email, password })
      addNotification('Registration successful! Please log in.', 'success')
      navigate('/login')
    } catch (err) {
      const errorMessage = err.response?.data?.message || 'Registration failed'
      setError(errorMessage)
      addNotification('Registration failed: ' + errorMessage, 'error')
      console.error('Registration error:', errorMessage)
    }
    setLoading(false)
  }

  return (
    <div className="flex justify-center items-center h-screen bg-gray-100">
      <div className="bg-white p-12 rounded-3xl shadow-2xl w-full max-w-md" role="main" aria-labelledby="register-title">
        <h1 id="register-title" className="text-4xl font-bold mb-8 text-center text-gray-800">Register</h1>

        {error && (
          <div
            role="alert"
            aria-live="assertive"
            className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg"
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} aria-labelledby="register-form-title">
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
              onChange={(e) => {
                setEmail(e.target.value)
                if (emailError) setEmailError('')
              }}
              className={`w-full p-4 border ${emailError ? 'border-red-500' : 'border-gray-300'} rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500`}
              required
              aria-invalid={!!emailError}
              aria-describedby={emailError ? 'email-error' : undefined}
            />
            {emailError && (
              <p
                id="email-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                {emailError}
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
              onChange={(e) => {
                setPassword(e.target.value)
                if (passwordError) setPasswordError('')
              }}
              className={`w-full p-4 border ${passwordError ? 'border-red-500' : 'border-gray-300'} rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500`}
              required
              aria-invalid={!!passwordError}
              aria-describedby={passwordError ? 'password-error' : undefined}
            />
            {password && <PasswordStrength password={password} />}
            {passwordError && (
              <p
                id="password-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                {passwordError}
              </p>
            )}
          </div>

          <div className="mb-6">
            <label
              htmlFor="confirm-password"
              className="block text-gray-700 mb-2"
            >
              Confirm Password
            </label>
            <input
              id="confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="w-full p-4 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500"
              required
              aria-invalid={error.includes('Passwords do not match')}
              aria-describedby={error.includes('Passwords do not match') ? 'confirm-password-error' : undefined}
            />
            {error.includes('Passwords do not match') && (
              <p
                id="confirm-password-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                Passwords do not match
              </p>
            )}
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-indigo-600 text-white py-4 rounded-xl hover:bg-indigo-700 transition text-xl font-bold disabled:opacity-50"
            aria-busy={loading}
          >
            {loading ? 'Registering...' : 'Register'}
          </button>
        </form>

        <div className="mt-6 text-center">
          <p className="text-gray-600">
            Already have an account?{' '}
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












