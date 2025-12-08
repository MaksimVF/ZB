







import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { useForm } from 'react-hook-form'
import { useNotification } from '../context/NotificationContext'

export default function Login() {
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm()
  const [error, setError] = useState('')
  const { login } = useAuth()
  const navigate = useNavigate()
  const { addNotification } = useNotification()

  const onSubmit = async (data) => {
    setError('')

    try {
      const success = await login(data.username, data.password)
      if (success) {
        addNotification('Login successful! Welcome back.', 'success')
        navigate('/admin')
      } else {
        setError('Invalid username or password')
        addNotification('Login failed. Please check your credentials.', 'error')
      }
    } catch (err) {
      setError('An unexpected error occurred. Please try again.')
      addNotification('Login error: ' + (err.message || 'Unknown error'), 'error')
      console.error('Login error:', err)
    }
  }

  return (
    <div className="flex justify-center items-center h-screen bg-gray-100 dark:bg-gray-900">
      <div className="bg-white dark:bg-gray-800 p-12 rounded-3xl shadow-2xl w-full max-w-md" role="main" aria-labelledby="login-title">
        <h1 id="login-title" className="text-4xl font-bold mb-8 text-center text-gray-800 dark:text-gray-100">Admin Login</h1>

        {error && (
          <div
            role="alert"
            aria-live="assertive"
            className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg dark:bg-red-900 dark:text-red-100"
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} aria-labelledby="login-form-title">
          <div className="mb-6">
            <label
              htmlFor="username"
              className="block text-gray-700 dark:text-gray-300 mb-2"
            >
              Username
            </label>
            <input
              id="username"
              type="text"
              {...register('username', { required: 'Username is required' })}
              className="w-full p-4 border border-gray-300 dark:border-gray-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-700 dark:text-white"
              required
              aria-invalid={errors.username ? 'true' : 'false'}
              aria-describedby={errors.username ? 'username-error' : undefined}
            />
            {errors.username && (
              <p
                id="username-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                {errors.username.message}
              </p>
            )}
          </div>

          <div className="mb-8">
            <label
              htmlFor="password"
              className="block text-gray-700 dark:text-gray-300 mb-2"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              {...register('password', { required: 'Password is required' })}
              className="w-full p-4 border border-gray-300 dark:border-gray-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-700 dark:text-white"
              required
              aria-invalid={errors.password ? 'true' : 'false'}
              aria-describedby={errors.password ? 'password-error' : undefined}
            />
            {errors.password && (
              <p
                id="password-error"
                className="text-red-500 text-sm mt-1"
                role="alert"
              >
                {errors.password.message}
              </p>
            )}
          </div>

          <button
            type="submit"
            className="w-full bg-indigo-600 text-white py-4 rounded-xl hover:bg-indigo-700 transition text-xl font-bold"
            disabled={isSubmitting}
            aria-busy={isSubmitting}
          >
            {isSubmitting ? 'Logging in...' : 'Login'}
          </button>
        </form>
      </div>
    </div>
  )
}







