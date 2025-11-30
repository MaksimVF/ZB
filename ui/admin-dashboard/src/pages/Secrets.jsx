





import { useEffect, useState } from 'react'
import { admin } from '../api'

export default function Secrets() {
  const [secrets, setSecrets] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin.secrets().then(r => {
      setSecrets(r.data)
      setLoading(false)
    })
  }, [])

  const saveSecret = async (name, value) => {
    await admin.saveSecret(name, value)
    alert('Секрет сохранен!')
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Секреты</h1>

      <div className="bg-white rounded-2xl shadow-xl p-8">
        <h2 className="text-2xl font-bold mb-6">API ключи провайдеров</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          {secrets.map(secret => (
            <div key={secret.name} className="bg-gray-50 p-6 rounded-xl">
              <h3 className="text-xl font-bold mb-4">{secret.name}</h3>

              <div className="mb-4">
                <label className="block text-gray-700 mb-2">Значение</label>
                <input
                  type="text"
                  value={secret.value}
                  onChange={(e) => {
                    const newSecrets = secrets.map(s =>
                      s.name === secret.name ? { ...s, value: e.target.value } : s
                    )
                    setSecrets(newSecrets)
                  }}
                  className="w-full p-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <button
                onClick={() => saveSecret(secret.name, secret.value)}
                className="bg-red-600 text-white px-6 py-3 rounded-lg hover:bg-red-700 transition"
              >
                Сохранить
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}





