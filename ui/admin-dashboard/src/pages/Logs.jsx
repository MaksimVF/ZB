






import { useEffect, useState } from 'react'
import { admin } from '../api'

export default function Logs() {
  const [logs, setLogs] = useState([])
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadLogs()
  }, [page])

  const loadLogs = () => {
    setLoading(true)
    admin.logs(page).then(r => {
      setLogs(r.data.logs)
      setLoading(false)
    })
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Логи</h1>

      <div className="bg-white rounded-2xl shadow-xl p-8">
        <div className="mb-6">
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            className="bg-gray-600 text-white px-6 py-3 rounded-lg mr-3"
            disabled={page === 1}
          >
            Предыдущая
          </button>
          <button
            onClick={() => setPage(p => p + 1)}
            className="bg-gray-600 text-white px-6 py-3 rounded-lg"
          >
            Следующая
          </button>
        </div>

        {loading ? (
          <div className="flex justify-center items-center h-64">Loading...</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-900 text-white">
                <tr>
                  <th className="p-4 text-left">Время</th>
                  <th className="p-4 text-left">Пользователь</th>
                  <th className="p-4 text-left">Модель</th>
                  <th className="p-4 text-left">Статус</th>
                  <th className="p-4 text-left">Токены</th>
                  <th className="p-4 text-left">Стоимость</th>
                </tr>
              </thead>
              <tbody>
                {logs.map(log => (
                  <tr key={log.id} className="border-t hover:bg-gray-50">
                    <td className="p-4">{new Date(log.timestamp).toLocaleString()}</td>
                    <td className="p-4">{log.user_id}</td>
                    <td className="p-4">{log.model}</td>
                    <td className="p-4">
                      <span className={`px-3 py-1 rounded-full text-white text-sm ${log.status === 'success' ? 'bg-green-600' : 'bg-red-600'}`}>
                        {log.status}
                      </span>
                    </td>
                    <td className="p-4">{log.tokens}</td>
                    <td className="p-4">${log.cost.toFixed(4)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}






