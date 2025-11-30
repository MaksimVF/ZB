





import { useEffect, useState } from 'react'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { admin } from '../api'

export default function RateLimits() {
  const [rateLimits, setRateLimits] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin.rateLimits().then(r => {
      setRateLimits(r.data)
      setLoading(false)
    })
  }, [])

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Rate Limits</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {rateLimits.map(limit => (
          <div key={limit.id} className="bg-white rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold mb-6">{limit.name}</h2>

            <div className="mb-6">
              <p className="text-gray-700 mb-2">Current: {limit.current}</p>
              <p className="text-gray-700 mb-2">Limit: {limit.limit}</p>
              <p className="text-gray-700">Window: {limit.window} seconds</p>
            </div>

            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={limit.history}>
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Line type="monotone" dataKey="requests" stroke="#8884d8" strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        ))}
      </div>
    </div>
  )
}





