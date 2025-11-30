






import { useQuery } from 'react-query'
import { admin } from '../api'
import { useState } from 'react'

export default function Logs() {
  const [page, setPage] = useState(1)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  const { data: logs, isLoading, refetch } = useQuery({
    queryKey: ['logs', page, startDate, endDate],
    queryFn: () => admin.logs(page, { startDate, endDate }),
    keepPreviousData: true,
  })

  const exportToCSV = () => {
    const csvContent = "data:text/csv;charset=utf-8,"
      + logs?.data?.logs?.map(log => Object.values(log).join(",")).join("\n")

    const encodedUri = encodeURI(csvContent)
    const link = document.createElement("a")
    link.setAttribute("href", encodedUri)
    link.setAttribute("download", "logs.csv")
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const applyFilters = () => {
    setPage(1)
    refetch()
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8 dark:text-white">Логи</h1>

      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
        {/* Filters */}
        <div className="mb-6 flex flex-col md:flex-row md:items-center md:justify-between space-y-4 md:space-y-0">
          <div className="flex space-x-4">
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white"
            />
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-700 dark:text-white"
            />
            <button
              onClick={applyFilters}
              className="bg-blue-600 text-white px-6 py-3 rounded-lg"
            >
              Применить
            </button>
          </div>

          <button
            onClick={exportToCSV}
            className="bg-green-600 text-white px-6 py-3 rounded-lg"
          >
            Экспорт в CSV
          </button>
        </div>

        {/* Pagination */}
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

        {isLoading ? (
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
                {logs?.data?.logs?.map(log => (
                  <tr key={log.id} className="border-t hover:bg-gray-50 dark:hover:bg-gray-700">
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






