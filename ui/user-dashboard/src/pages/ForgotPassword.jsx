












import { useState } from 'react'
import { auth } from '../api'

export default function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setMessage('')

    setLoading(true)
    try {
      await auth.forgotPassword({ email })
      setMessage('Password reset link sent to your email')
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to send reset link')
    }
    setLoading(false)
  }

  return (
    <div className="flex justify-center items-center h-screen bg-gray-100">
      <div className="bg-white p-12 rounded-3xl shadow-2xl w-full max-w-md">
        <h1 className="text-4xl font-bold mb-8 text-center text-gray-800">Forgot Password</h1>

        {error && <div className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg">{error}</div>}
        {message && <div className="mb-6 p-4 bg-green-100 text-green-700 rounded-lg">{message}</div>}

        <form onSubmit={handleSubmit}>
          <div className="mb-6">
            <label className="block text-gray-700 mb-2">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full p-4 border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-indigo-600 text-white py-4 rounded-xl hover:bg-indigo-700 transition text-xl font-bold disabled:opacity-50"
          >
            {loading ? 'Sending...' : 'Send Reset Link'}
          </button>
        </form>

        <div className="mt-6 text-center">
          <p className="text-gray-600">Remember your password? <a href="/login" className="text-indigo-600 hover:underline">Login</a></p>
        </div>
      </div>
    </div>
  )
}














