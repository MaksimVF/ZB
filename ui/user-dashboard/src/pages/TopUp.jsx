







import { useState } from 'react'
import { billing } from '../api'

const amounts = [10, 25, 50, 100, 250, 500]

export default function TopUp() {
  const [loading, setLoading] = useState(false)

  const pay = async (amount) => {
    setLoading(true)
    try {
      const res = await billing.topup(amount)
      window.location.href = res.data.url
    } catch (e) {
      alert("Ошибка оплаты")
    }
    setLoading(false)
  }

  return (
    <div className="max-w-2xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Пополнение баланса</h1>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-6">
        {amounts.map(a => (
          <button
            key={a}
            onClick={() => pay(a)}
            disabled={loading}
            className="bg-gradient-to-br from-blue-500 to-purple-600 text-white p-8 rounded-2xl text-2xl font-bold hover:scale-105 transition disabled:opacity-50"
          >
            ${a}
          </button>
        ))}
      </div>

      <div className="mt-10 bg-gray-100 p-8 rounded-xl">
        <h2 className="text-2xl font-bold mb-4">Или введите свою сумму</h2>
        <input type="number" placeholder="100" className="px-4 py-3 rounded-lg text-xl w-full" />
        <button className="mt-4 bg-black text-white px-8 py-4 rounded-lg font-bold">
          Оплатить
        </button>
      </div>
    </div>
  )
}








