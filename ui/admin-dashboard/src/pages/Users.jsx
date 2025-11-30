




import { useEffect, useState } from 'react'
import { admin } from '../api'

export default function Users() {
  const [users, setUsers] = useState([])

  useEffect(() => {
    admin.users().then(r => setUsers(r.data))
  }, [])

  const addBonus = async (userId) => {
    const amount = prompt("Сколько долларов добавить?")
    if (amount) {
      await admin.adjustBalance(userId, parseFloat(amount), "admin_bonus")
      alert("Бонус начисления!")
    }
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Пользователи</h1>
      <div className="bg-white rounded-2xl shadow-xl overflow-hidden">
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
            {users.map(u => (
              <tr key={u.id} className="border-t hover:bg-gray-50">
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




