









import { useEffect, useState } from 'react'
import { billing } from '../api'

export default function History() {
  const [history, setHistory] = useState([])

  useEffect(() => {
    const load = async () => {
      const res = await billing.history()
      setHistory(res.data.transactions)
    }
    load()
  }, [])

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">История транзакций</h1>

      <div className="bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Все транзакции</h2>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-900 text-white">
              <tr>
                <th className="p-4 text-left">Дата</th>
                <th className="p-4 text-left">Тип</th>
                <th className="p-4 text-left">Сумма</th>
                <th className="p-4 text-left">Статус</th>
              </tr>
            </thead>
            <tbody>
              {history.map(t => (
                <tr key={t.id} className="border-t hover:bg-gray-50">
                  <td className="p-4">{new Date(t.timestamp).toLocaleString()}</td>
                  <td className="p-4">{t.type}</td>
                  <td className="p-4">${t.amount_usd.toFixed(2)}</td>
                  <td className="p-4">
                    <span className={`px-3 py-1 rounded-full text-white text-sm ${t.status === 'success' ? 'bg-green-600' : 'bg-red-600'}`}>
                      {t.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}











