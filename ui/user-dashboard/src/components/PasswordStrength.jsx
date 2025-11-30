











import { useState, useEffect } from 'react'

const PasswordStrength = ({ password }) => {
  const [strength, setStrength] = useState(0)
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (!password) {
      setStrength(0)
      setMessage('')
      return
    }

    let score = 0

    // Length check
    if (password.length >= 8) score += 1
    if (password.length >= 12) score += 1

    // Character variety
    if (/[A-Z]/.test(password)) score += 1
    if (/[a-z]/.test(password)) score += 1
    if (/[0-9]/.test(password)) score += 1
    if (/[^A-Za-z0-9]/.test(password)) score += 1

    // Common patterns
    if (password.length > 0 && score > 0) {
      const commonPatterns = ['123456', 'password', 'qwerty', 'abc123']
      if (commonPatterns.some(p => password.toLowerCase().includes(p))) {
        score = Math.max(0, score - 2)
      }
    }

    // Set strength level
    if (score <= 2) {
      setStrength(1)
      setMessage('Weak')
    } else if (score <= 4) {
      setStrength(2)
      setMessage('Medium')
    } else {
      setStrength(3)
      setMessage('Strong')
    }
  }, [password])

  const getColor = () => {
    switch (strength) {
      case 1: return 'bg-red-500'
      case 2: return 'bg-yellow-500'
      case 3: return 'bg-green-500'
      default: return 'bg-gray-300'
    }
  }

  return (
    <div className="mt-2">
      <div className="flex items-center justify-between mb-1">
        <span className="text-sm">Password strength: {message}</span>
      </div>
      <div className="h-2 bg-gray-200 rounded-full">
        <div className={`h-full rounded-full ${getColor()}`} style={{ width: `${(strength / 3) * 100}%` }}></div>
      </div>
    </div>
  )
}

export default PasswordStrength











