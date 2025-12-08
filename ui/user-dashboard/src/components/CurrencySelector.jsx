












import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'

const currencies = [
  { code: 'USD', name: 'US Dollar', symbol: '$' },
  { code: 'EUR', name: 'Euro', symbol: '€' },
  { code: 'RUB', name: 'Russian Ruble', symbol: '₽' },
  { code: 'CNY', name: 'Chinese Yuan', symbol: '¥' },
  { code: 'GBP', name: 'British Pound', symbol: '£' },
  { code: 'JPY', name: 'Japanese Yen', symbol: '¥' }
]

export default function CurrencySelector() {
  const { currency, updatePreferences } = useAuth()
  const [showModal, setShowModal] = useState(false)
  const [selectedCurrency, setSelectedCurrency] = useState(currency)

  useEffect(() => {
    setSelectedCurrency(currency)
  }, [currency])

  const handleSave = async () => {
    await updatePreferences({ currency: selectedCurrency })
    setShowModal(false)
  }

  const currentCurrency = currencies.find(c => c.code === currency) || currencies[0]

  return (
    <div>
      <button
        onClick={() => setShowModal(true)}
        className="flex items-center space-x-2 px-4 py-2 bg-gray-200 rounded-lg hover:bg-gray-300"
      >
        <span>{currentCurrency.symbol}</span>
        <span>{currentCurrency.code}</span>
      </button>

      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white p-8 rounded-xl shadow-2xl max-w-md w-full">
            <h2 className="text-2xl font-bold mb-6">Select Currency</h2>

            <div className="space-y-4 mb-6">
              {currencies.map(c => (
                <label key={c.code} className="flex items-center space-x-3">
                  <input
                    type="radio"
                    name="currency"
                    value={c.code}
                    checked={selectedCurrency === c.code}
                    onChange={() => setSelectedCurrency(c.code)}
                    className="form-radio text-indigo-600"
                  />
                  <span>{c.name} ({c.symbol})</span>
                </label>
              ))}
            </div>

            <div className="flex justify-end space-x-4">
              <button
                onClick={() => setShowModal(false)}
                className="px-6 py-2 bg-gray-200 rounded-lg hover:bg-gray-300"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="px-6 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700"
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}















