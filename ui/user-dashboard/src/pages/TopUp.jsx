







import { useState } from 'react'
import { billing } from '../api'

const amounts = [10, 25, 50, 100, 250, 500]

export default function TopUp() {
  const [loading, setLoading] = useState(false)
  const [customAmount, setCustomAmount] = useState('')

  const pay = async (amount) => {
    setLoading(true)
    try {
      const res = await billing.topup(amount)
      // Redirect to YuKassa payment page
      window.location.href = res.data.url
    } catch (e) {
      alert("Ошибка оплаты через ЮКассу")
    }
    setLoading(false)
  }

  const handleCustomPayment = () => {
    const amount = parseFloat(customAmount)
    if (isNaN(amount) || amount <= 0) {
      alert("Пожалуйста, введите корректную сумму")
      return
    }
    pay(amount)
  }

  return (
    <div className="max-w-2xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Пополнение баланса</h1>

      <div className="bg-yellow-100 p-6 rounded-xl mb-8">
        <h2 className="text-xl font-bold mb-2">Оплата через ЮКассу</h2>
        <p className="text-gray-700">Все платежи обрабатываются через безопасную систему ЮКассы</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-6">
        {amounts.map(a => (
          <button
            key={a}
            onClick={() => pay(a)}
            disabled={loading}
            className="bg-gradient-to-br from-yellow-500 to-orange-600 text-white p-8 rounded-2xl text-2xl font-bold hover:scale-105 transition disabled:opacity-50"
          >
            ${a}
          </button>
        ))}
      </div>

      <div className="mt-10 bg-gray-100 p-8 rounded-xl">
        <h2 className="text-2xl font-bold mb-4">Или введите свою сумму</h2>
        <input
          type="number"
          placeholder="100"
          value={customAmount}
          onChange={(e) => setCustomAmount(e.target.value)}
          className="px-4 py-3 rounded-lg text-xl w-full"
        />
        <button
          onClick={handleCustomPayment}
          disabled={loading}
          className="mt-4 bg-yellow-600 text-white px-8 py-4 rounded-lg font-bold hover:bg-yellow-700 disabled:opacity-50"
        >
          Оплатить через ЮКассу
        </button>
      </div>
    </div>
  )
}








