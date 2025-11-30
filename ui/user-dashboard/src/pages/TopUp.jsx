







import { useState, useEffect } from 'react'
import { billing } from '../api'

const amounts = [10, 25, 50, 100, 250, 500]

export default function TopUp() {
  const [loading, setLoading] = useState(false)
  const [customAmount, setCustomAmount] = useState('')
  const [paymentMethod, setPaymentMethod] = useState('yukassa')
  const [paymentHistory, setPaymentHistory] = useState([])
  const [subscriptionPlans, setSubscriptionPlans] = useState([])

  useEffect(() => {
    const loadHistory = async () => {
      const res = await billing.history()
      setPaymentHistory(res.data.transactions)
    }
    loadHistory()

    // Load subscription plans
    const loadPlans = async () => {
      const res = await billing.getSubscriptionPlans()
      setSubscriptionPlans(res.data.plans)
    }
    loadPlans()
  }, [])

  const pay = async (amount) => {
    setLoading(true)
    try {
      const res = await billing.topup(amount, { payment_method: paymentMethod })
      // Redirect to payment provider
      window.location.href = res.data.url
    } catch (e) {
      alert(`Ошибка оплаты через ${paymentMethod === 'yukassa' ? 'ЮКассу' : paymentMethod}`)
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

  const subscribe = async (planId) => {
    setLoading(true)
    try {
      const res = await billing.subscribe(planId)
      window.location.href = res.data.url
    } catch (e) {
      alert("Ошибка подписки")
    }
    setLoading(false)
  }

  return (
    <div className="max-w-4xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Пополнение баланса</h1>

      {/* Payment Method Selection */}
      <div className="mb-8">
        <h2 className="text-2xl font-bold mb-4">Выберите способ оплаты</h2>
        <div className="flex space-x-4">
          <button
            onClick={() => setPaymentMethod('yukassa')}
            className={`px-6 py-3 rounded-lg ${paymentMethod === 'yukassa' ? 'bg-yellow-600 text-white' : 'bg-gray-200'}`}
          >
            ЮКасса
          </button>
          <button
            onClick={() => setPaymentMethod('stripe')}
            className={`px-6 py-3 rounded-lg ${paymentMethod === 'stripe' ? 'bg-purple-600 text-white' : 'bg-gray-200'}`}
          >
            Stripe
          </button>
          <button
            onClick={() => setPaymentMethod('paypal')}
            className={`px-6 py-3 rounded-lg ${paymentMethod === 'paypal' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
          >
            PayPal
          </button>
        </div>
      </div>

      {/* Payment Options */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {/* One-time Payment */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Разовый платеж</h2>

          <div className="grid grid-cols-2 gap-4 mb-6">
            {amounts.map(a => (
              <button
                key={a}
                onClick={() => pay(a)}
                disabled={loading}
                className="bg-gradient-to-br from-yellow-500 to-orange-600 text-white p-4 rounded-xl text-xl font-bold hover:scale-105 transition disabled:opacity-50"
              >
                ${a}
              </button>
            ))}
          </div>

          <div className="mt-6">
            <h3 className="text-xl font-bold mb-2">Или введите свою сумму</h3>
            <input
              type="number"
              placeholder="100"
              value={customAmount}
              onChange={(e) => setCustomAmount(e.target.value)}
              className="px-4 py-3 rounded-lg text-xl w-full mb-4"
            />
            <button
              onClick={handleCustomPayment}
              disabled={loading}
              className="w-full bg-yellow-600 text-white px-6 py-3 rounded-lg font-bold hover:bg-yellow-700 disabled:opacity-50"
            >
              Оплатить ${customAmount || '0'}
            </button>
          </div>
        </div>

        {/* Subscription Plans */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Подписки</h2>

          {subscriptionPlans.length > 0 ? (
            <div className="space-y-4">
              {subscriptionPlans.map(plan => (
                <div key={plan.id} className="p-4 border border-gray-200 rounded-lg">
                  <h3 className="text-xl font-bold">{plan.name}</h3>
                  <p className="text-gray-600 mb-2">{plan.description}</p>
                  <div className="flex justify-between items-center">
                    <span className="text-2xl font-bold">${plan.price}/month</span>
                    <button
                      onClick={() => subscribe(plan.id)}
                      disabled={loading}
                      className="bg-indigo-600 text-white px-4 py-2 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                    >
                      Подписаться
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p>No subscription plans available at the moment.</p>
          )}
        </div>
      </div>

      {/* Payment History */}
      <div className="mt-12 bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">История платежей</h2>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-900 text-white">
              <tr>
                <th className="p-4 text-left">Дата</th>
                <th className="p-4 text-left">Тип</th>
                <th className="p-4 text-left">Сумма</th>
                <th className="p-4 text-left">Статус</th>
                <th className="p-4 text-left">Провайдер</th>
              </tr>
            </thead>
            <tbody>
              {paymentHistory.map(t => (
                <tr key={t.id} className="border-t hover:bg-gray-50">
                  <td className="p-4">{new Date(t.timestamp).toLocaleString()}</td>
                  <td className="p-4">{t.type}</td>
                  <td className="p-4">${t.amount_usd.toFixed(2)}</td>
                  <td className="p-4">
                    <span className={`px-3 py-1 rounded-full text-white text-sm ${t.status === 'success' ? 'bg-green-600' : 'bg-red-600'}`}>
                      {t.status}
                    </span>
                  </td>
                  <td className="p-4">{t.provider}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}








