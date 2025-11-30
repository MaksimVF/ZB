









import { useEffect, useState } from 'react'
import { settings } from '../api'

export default function Settings() {
  const [config, setConfig] = useState({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const load = async () => {
      const res = await settings.get()
      setConfig(res.data)
      setLoading(false)
    }
    load()
  }, [])

  const save = async () => {
    await settings.save(config)
    alert('Настройки сохранены!')
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Настройки</h1>

      <div className="bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Конфигурация</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          <div>
            <h3 className="text-xl font-bold mb-4">Тарифный план</h3>
            <select
              value={config.plan}
              onChange={(e) => setConfig({ ...config, plan: e.target.value })}
              className="w-full p-3 border border-gray-300 rounded-lg"
            >
              <option value="basic">Basic</option>
              <option value="pro">Pro</option>
              <option value="enterprise">Enterprise</option>
            </select>
          </div>

          <div>
            <h3 className="text-xl font-bold mb-4">Провайдеры</h3>
            <div className="space-y-2">
              {config.providers?.map(p => (
                <div key={p} className="flex items-center">
                  <input
                    type="checkbox"
                    checked={config.enabled_providers?.includes(p)}
                    onChange={(e) => {
                      const newProviders = e.target.checked
                        ? [...config.enabled_providers, p]
                        : config.enabled_providers.filter(ep => ep !== p)
                      setConfig({ ...config, enabled_providers: newProviders })
                    }}
                    className="mr-2"
                  />
                  {p}
                </div>
              ))}
            </div>
          </div>
        </div>

        <button
          onClick={save}
          className="mt-8 bg-blue-600 text-white px-8 py-4 rounded-lg hover:bg-blue-700 transition"
        >
          Сохранить настройки
        </button>
      </div>
    </div>
  )
}










