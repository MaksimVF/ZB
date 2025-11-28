import { useEffect, useState } from 'react'
import { LineChart, Line, BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { billing } from '../api'

export default function Dashboard() {
  const [balance, setBalance] = useState(0)
  const [usage, setUsage] = useState([])
  const [topModels, setTopModels] = useState([])

  useEffect(() => {
    const load = async () => {
      const bal = await billing.balance()
      const hist = await billing.history()
      const usg = await billing.usage()

      setBalance(bal.data.balance_usd)
      setUsage(usg.data.last_30_days)
      setTopModels(usg.data.top_models)
    }
    load()
    const i = setInterval(load, 30_000)
    return () => clearInterval(i)
  }, [])

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Личный кабинет</h1>

      {/* Баланс */}
      <div className="bg-gradient-to-r from-blue-600 to-purple-700 text-white p-10 rounded-2xl mb-10">
        <div className="text-6xl font-bold">${balance.toFixed(2)}</div>
        <div className="text-xl opacity-90">на вашем счету</div>
        <a href="/topup" className="mt-6 inline-block bg-white text-blue-600 px-8 py-4 rounded-xl font-bold">
          Пополнить баланс
        </a>
      </div>

      {/* Расходы за месяц */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Расходы за 30 дней</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={usage}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
              <Line type="monotone" dataKey="cost" stroke="#8b5cf6" strokeWidth={3} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Топ моделей</h2>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={topModels}>
              <XAxis dataKey="model" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="cost" fill="#3b82f6" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Быстрые действия */}
      <div className="mt-10 grid grid-cols-2 md:grid-cols-4 gap-6">
        <a href="/usage" className="bg-gray-100 p-6 rounded-xl text-center hover:bg-gray-200 transition">
          <div className="text-3xl">Chart</div>
          <div className="font-bold">Статистика</div>
        </a>
        <a href="/settings" className="bg-gray-100 p-6 rounded-xl text-center hover:bg-gray-200 transition">
          <div className="text-3xl">Gear</div>
          <div className="font-bold">Настройки</div>
        </a>
        <a href="/history" className="bg-gray-100 p-6 rounded-xl text-center hover:bg-gray-200 transition">
          <div className="text-3xl">History</div>
          <div className="font-bold">История</div>
        </a>
        <a href="/topup" className="bg-green-600 text-white p-6 rounded-xl text-center hover:bg-green-700 transition">
          <div className="text-3xl">Plus</div>
          <div className="font-bold">Пополнить</div>
        </a>
      </div>
    </div>
  )
}
