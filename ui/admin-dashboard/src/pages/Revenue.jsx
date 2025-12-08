







import { useQuery } from 'react-query'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts'
import { admin } from '../api'
import { useState } from 'react'

export default function Revenue() {
  const [period, setPeriod] = useState('monthly')

  const { data: revenueData, isLoading } = useQuery({
    queryKey: ['revenue', period],
    queryFn: () => admin.revenue({ period }),
  })

  if (isLoading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8 dark:text-white">Доход</h1>

      {/* Period selection */}
      <div className="mb-8">
        <div className="flex space-x-4">
          <button
            onClick={() => setPeriod('daily')}
            className={`px-6 py-3 rounded-lg ${period === 'daily' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
          >
            Дневной
          </button>
          <button
            onClick={() => setPeriod('weekly')}
            className={`px-6 py-3 rounded-lg ${period === 'weekly' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
          >
            Недельный
          </button>
          <button
            onClick={() => setPeriod('monthly')}
            className={`px-6 py-3 rounded-lg ${period === 'monthly' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
          >
            Месячный
          </button>
          <button
            onClick={() => setPeriod('forecast')}
            className={`px-6 py-3 rounded-lg ${period === 'forecast' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
          >
            Прогноз
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {period === 'daily' && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold mb-6 dark:text-white">Доход по дням</h2>
            <ResponsiveContainer width="100%" height={400}>
              <LineChart data={revenueData.daily}>
                <XAxis dataKey="date" />
                <YAxis />
                <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
                <Line type="monotone" dataKey="revenue" stroke="#10b981" strokeWidth={4} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}

        {period === 'weekly' && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold mb-6 dark:text-white">Доход по неделям</h2>
            <ResponsiveContainer width="100%" height={400}>
              <BarChart data={revenueData.weekly}>
                <XAxis dataKey="week" />
                <YAxis />
                <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
                <Bar dataKey="revenue" fill="#8b5cf6" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}

        {period === 'monthly' && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold mb-6 dark:text-white">Доход по месяцам</h2>
            <ResponsiveContainer width="100%" height={400}>
              <LineChart data={revenueData.monthly}>
                <XAxis dataKey="month" />
                <YAxis />
                <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
                <Line type="monotone" dataKey="revenue" stroke="#f59e0b" strokeWidth={4} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}

        {period === 'forecast' && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold mb-6 dark:text-white">Прогноз</h2>
            <ResponsiveContainer width="100%" height={400}>
              <LineChart data={revenueData.forecast}>
                <XAxis dataKey="date" />
                <YAxis />
                <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
                <Line type="monotone" dataKey="revenue" stroke="#06b6d4" strokeWidth={4} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  )
}







