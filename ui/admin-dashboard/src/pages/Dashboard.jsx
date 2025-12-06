



import { useQuery } from 'react-query'
import { LineChart, Line, BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { admin } from '../api'

export default function Dashboard() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: admin.stats,
    refetchInterval: 60000, // 1 minute
  })

  const { data: revenueData, isLoading: revenueLoading } = useQuery({
    queryKey: ['revenue'],
    queryFn: admin.revenue,
  })

  if (statsLoading || revenueLoading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  const revenue = revenueData?.data?.last_30_days || []
  const topModels = revenueData?.data?.top_models || []

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-5xl font-bold mb-10 text-gray-800 dark:text-gray-100">Админ-панель</h1>

      {/* Key metrics */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-8 mb-12">
        <div className="bg-gradient-to-br from-green-500 to-emerald-600 text-white p-8 rounded-2xl">
          <div className="text-4xl font-bold">${stats?.data?.total_revenue_usd?.toFixed(2) || 0}</div>
          <div className="text-xl opacity-90">Доход всего</div>
        </div>
        <div className="bg-gradient-to-br from-blue-500 to-cyan-600 text-white p-8 rounded-2xl">
          <div className="text-4xl font-bold">{stats?.data?.active_users || 0}</div>
          <div className="text-xl opacity-90">Активных пользователей</div>
        </div>
        <div className="bg-gradient-to-br from-purple-500 to-pink-600 text-white p-8 rounded-2xl">
          <div className="text-4xl font-bold">{stats?.data?.requests_today || 0}</div>
          <div className="text-xl opacity-90">Запросов сегодня</div>
        </div>
        <div className="bg-gradient-to-br from-orange-500 to-red-600 text-white p-8 rounded-2xl">
          <div className="text-4xl font-bold">${stats?.data?.revenue_today?.toFixed(2) || 0}</div>
          <div className="text-xl opacity-90">Доход сегодня</div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl">
          <h2 className="text-3xl font-bold mb-6 dark:text-white">Доход за 30 дней</h2>
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={revenue}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Line type="monotone" dataKey="revenue" stroke="#10b981" strokeWidth={4} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl">
          <h2 className="text-3xl font-bold mb-6 dark:text-white">Лучшие модели по доходу</h2>
          <ResponsiveContainer width="100%" height={400}>
            <BarChart data={topModels}>
              <XAxis dataKey="model" angle={-45} textAnchor="end" height={100} />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Bar dataKey="revenue" fill="#8b5cf6" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Quick links */}
      <div className="mt-12 grid grid-cols-2 md:grid-cols-4 gap-6">
        <a href="/admin/users" className="bg-gray-800 text-white p-8 rounded-2xl text-center hover:bg-gray-900 transition text-2xl font-bold">Пользователи</a>
        <a href="/admin/pricing" className="bg-indigo-600 text-white p-8 rounded-2xl text-center hover:bg-indigo-700 transition text-2xl font-bold">Тарифы</a>
        <a href="/admin/secrets" className="bg-red-600 text-white p-8 rounded-2xl text-center hover:bg-red-700 transition text-2xl font-bold">Секреты</a>
        <a href="/admin/logs" className="bg-teal-600 text-white p-8 rounded-2xl text-center hover:bg-teal-700 transition text-2xl font-bold">Логи</a>
        <a href="/admin/routing" className="bg-yellow-600 text-white p-8 rounded-2xl text-center hover:bg-yellow-700 transition text-2xl font-bold">Маршрутизация</a>
      </div>
    </div>
  )
}



