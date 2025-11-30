







import { useEffect, useState } from 'react'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts'
import { admin } from '../api'

export default function Revenue() {
  const [revenueData, setRevenueData] = useState({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin.revenue().then(r => {
      setRevenueData(r.data)
      setLoading(false)
    })
  }, [])

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Доход</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-2xl font-bold mb-6">Доход по дням</h2>
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={revenueData.daily}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Line type="monotone" dataKey="revenue" stroke="#10b981" strokeWidth={4} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-2xl font-bold mb-6">Доход по неделям</h2>
          <ResponsiveContainer width="100%" height={400}>
            <BarChart data={revenueData.weekly}>
              <XAxis dataKey="week" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Bar dataKey="revenue" fill="#8b5cf6" />
            </BarChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-2xl font-bold mb-6">Доход по месяцам</h2>
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={revenueData.monthly}>
              <XAxis dataKey="month" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Line type="monotone" dataKey="revenue" stroke="#f59e0b" strokeWidth={4} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-2xl shadow-xl p-8">
          <h2 className="text-2xl font-bold mb-6">Прогноз</h2>
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={revenueData.forecast}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Line type="monotone" dataKey="revenue" stroke="#06b6d4" strokeWidth={4} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}







