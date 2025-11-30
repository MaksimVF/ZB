




import { useQuery } from 'react-query'
import { admin } from '../api'
import { useState } from 'react'

export default function Users() {
  const { data: users, isLoading, refetch } = useQuery({
    queryKey: ['users'],
    queryFn: admin.users,
  })

  const [searchTerm, setSearchTerm] = useState('')
  const [filter, setFilter] = useState('all')

  const addBonus = async (userId) => {
    const amount = prompt("Сколько долларов добавить?")
    if (amount) {
      await admin.adjustBalance(userId, parseFloat(amount), "admin_bonus")
      alert("Бонус начисления!")
      refetch()
    }
  }

  const filteredUsers = users?.data?.filter(user => {
    const matchesSearch = user.email.includes(searchTerm) || user.id.includes(searchTerm)
    if (filter === 'all') return matchesSearch
    if (filter === 'active') return matchesSearch && user.active
    if (filter === 'inactive') return matchesSearch && !user.active
    return matchesSearch
  }) || []

  if (isLoading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8 dark:text-white">Пользователи</h1>

      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl overflow-hidden">
        {/* Search and filters */}
        <div className="p-6 mb-4 flex flex-col md:flex-row md:items-center md:justify-between space-y-4 md:space-y-0">
          <input
            type="text"
            placeholder="Поиск по email или ID..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white w-full md:w-1/3"
          />

          <div className="flex space-x-4">
            <button
              onClick={() => setFilter('all')}
              className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
            >
              Все
            </button>
            <button
              onClick={() => setFilter('active')}
              className={`px-4 py-2 rounded-lg ${filter === 'active' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
            >
              Активные
            </button>
            <button
              onClick={() => setFilter('inactive')}
              className={`px-4 py-2 rounded-lg ${filter === 'inactive' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-600'}`}
            >
              Заблокированные
            </button>
          </div>
        </div>

        <table className="w-full">
          <thead className="bg-gray-900 text-white">
            <tr>
              <th className="p-6 text-left">Идентификатор</th>
              <th className="p-6 text-left">Электронная почта/ключ API</th>
              <th className="p-6 text-left">Баланс</th>
              <th className="p-6 text-left">Статус</th>
              <th className="p-6 text-left">Действия</th>
            </tr>
          </thead>
          <tbody>
            {filteredUsers.map(u => (
              <tr key={u.id} className="border-t hover:bg-gray-50 dark:hover:bg-gray-700">
                <td className="p-6">{u.id}</td>
                <td className="p-6">{u.email || u.api_key_prefix}</td>
                <td className="p-6 font-bold">${u.balance_usd?.toFixed(2)}</td>
                <td className="p-6">
                  <span className={`px-4 py-2 rounded-full text-white ${u.active ? 'bg-green-600' : 'bg-red-600'}`}>
                    {u.active ? 'Активен' : 'Заблокирован'}
                  </span>
                </td>
                <td className="p-6">
                  <button onClick={() => addBonus(u.id)} className="bg-green-600 text-white px-6 py-3 rounded-lg mr-3">
                    + Бонус
                  </button>
                  <button className="bg-gray-600 text-white px-6 py-3 rounded-lg">
                    Заблокировать
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}




