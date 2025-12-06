









import { useEffect, useState } from 'react'
import { settings } from '../api'
import { useNotification } from '../context/NotificationContext'

export default function Settings() {
  const [config, setConfig] = useState({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const { addNotification } = useNotification()

  useEffect(() => {
    const load = async () => {
      try {
        const res = await settings.get()
        setConfig(res.data)
      } catch (err) {
        setError('Failed to load settings: ' + (err.response?.data?.message || err.message))
        addNotification('Failed to load settings', 'error')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [addNotification])

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      await settings.save(config)
      addNotification('Settings saved successfully!', 'success')
    } catch (err) {
      const errorMessage = err.response?.data?.message || 'Failed to save settings'
      setError(errorMessage)
      addNotification('Failed to save settings: ' + errorMessage, 'error')
      console.error('Settings save error:', errorMessage)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center items-center h-screen" aria-busy="true" aria-live="polite">
        <div className="text-center">
          <div className="spinner mb-4"></div>
          <p>Loading settings...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto p-8" role="main" aria-labelledby="settings-title">
      <h1 id="settings-title" className="text-4xl font-bold mb-8">Настройки</h1>

      {error && (
        <div
          role="alert"
          aria-live="assertive"
          className="mb-6 p-4 bg-red-100 text-red-700 rounded-lg"
        >
          {error}
        </div>
      )}

      <div className="bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Конфигурация</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          <div>
            <h3 className="text-xl font-bold mb-4">Тарифный план</h3>
            <select
              value={config.plan}
              onChange={(e) => setConfig({ ...config, plan: e.target.value })}
              className="w-full p-3 border border-gray-300 rounded-lg"
              aria-label="Select pricing plan"
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
                    id={`provider-${p}`}
                    aria-label={`Toggle provider ${p}`}
                  />
                  <label htmlFor={`provider-${p}`}>{p}</label>
                </div>
              ))}
            </div>
          </div>
        </div>

        <button
          onClick={save}
          className="mt-8 bg-blue-600 text-white px-8 py-4 rounded-lg hover:bg-blue-700 transition"
          disabled={saving}
          aria-busy={saving}
        >
          {saving ? 'Сохранение...' : 'Сохранить настройки'}
        </button>
      </div>
    </div>
  )
}










