





import { useEffect, useState } from 'react'
import { admin } from '../api'

export default function Pricing() {
  const [pricing, setPricing] = useState({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin.pricing().then(r => {
      setPricing(r.data)
      setLoading(false)
    })
  }, [])

  const savePricing = async () => {
    await admin.savePricing(pricing)
    alert('Цены сохранены!')
  }

  const updatePrice = (model, type, value) => {
    setPricing(prev => ({
      ...prev,
      [model]: {
        ...prev[model],
        [type]: parseFloat(value)
      }
    }))
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Тарифы</h1>

      <div className="bg-white rounded-2xl shadow-xl p-8">
        <h2 className="text-2xl font-bold mb-6">Цены на модели</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          {Object.entries(pricing).map(([model, prices]) => (
            <div key={model} className="bg-gray-50 p-6 rounded-xl">
              <h3 className="text-xl font-bold mb-4">{model}</h3>

              <div className="mb-4">
                <label className="block text-gray-700 mb-2">Цена за чат ($/1K токенов)</label>
                <input
                  type="number"
                  step="0.01"
                  value={prices.chat || 0}
                  onChange={(e) => updatePrice(model, 'chat', e.target.value)}
                  className="w-full p-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div className="mb-4">
                <label className="block text-gray-700 mb-2">Цена за эмбеддинги ($/1K токенов)</label>
                <input
                  type="number"
                  step="0.01"
                  value={prices.embeddings || 0}
                  onChange={(e) => updatePrice(model, 'embeddings', e.target.value)}
                  className="w-full p-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>
            </div>
          ))}
        </div>

        <div className="mt-8">
          <button
            onClick={savePricing}
            className="bg-indigo-600 text-white px-8 py-4 rounded-lg hover:bg-indigo-700 transition text-xl font-bold"
          >
            Сохранить цены
          </button>
        </div>
      </div>
    </div>
  )
}





