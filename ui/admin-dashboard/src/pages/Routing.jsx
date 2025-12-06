
import { useState, useEffect } from 'react'
import { admin } from '../api'

// Common region codes
const REGIONS = [
  { code: 'us', name: 'United States' },
  { code: 'eu', name: 'Europe' },
  { code: 'ru', name: 'Russia' },
  { code: 'cn', name: 'China' },
  { code: 'br', name: 'Brazil' },
  { code: 'in', name: 'India' },
  { code: 'jp', name: 'Japan' },
  { code: 'au', name: 'Australia' },
  { code: 'es', name: 'Spain' },
  { code: 'de', name: 'Germany' },
  { code: 'fr', name: 'France' },
  { code: 'uk', name: 'United Kingdom' }
]

export default function Routing() {
  const [policy, setPolicy] = useState({
    default_strategy: 'round_robin',
    enable_geo_routing: false,
    enable_load_balancing: false,
    enable_model_specific: false,
    strategy_config: {}
  })
  const [heads, setHeads] = useState([])
  const [regionFilter, setRegionFilter] = useState('')
  const [newHead, setNewHead] = useState({
    head_id: '',
    endpoint: '',
    region: '',
    model_type: '',
    version: '',
    metadata: {}
  })

  // Filter heads by region
  const filteredHeads = regionFilter
    ? heads.filter(head => head.region === regionFilter)
    : heads

  useEffect(() => {
    loadRoutingPolicy()
    loadHeadServices()
  }, [])

  const loadRoutingPolicy = async () => {
    try {
      const response = await admin.routingPolicy()
      setPolicy(response.data)
    } catch (error) {
      console.error('Error loading policy:', error)
    }
  }

  const loadHeadServices = async () => {
    try {
      const response = await admin.headServices()
      setHeads(response.data)
    } catch (error) {
      console.error('Error loading heads:', error)
    }
  }

  const updateRoutingPolicy = async (e) => {
    e.preventDefault()
    try {
      await admin.updateRoutingPolicy(policy)
      alert('Policy updated successfully')
    } catch (error) {
      console.error('Error updating policy:', error)
      alert('Failed to update policy')
    }
  }

  const registerHead = async (e) => {
    e.preventDefault()
    try {
      await admin.registerHead(newHead)
      alert('Head registered successfully')
      loadHeadServices() // Refresh the list
      setNewHead({
        head_id: '',
        endpoint: '',
        region: '',
        model_type: '',
        version: '',
        metadata: {}
      })
    } catch (error) {
      console.error('Error registering head:', error)
      alert('Failed to register head')
    }
  }

  const handlePolicyChange = (e) => {
    const { name, value, type, checked } = e.target
    setPolicy(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value
    }))
  }

  const handleNewHeadChange = (e) => {
    const { name, value } = e.target
    setNewHead(prev => ({
      ...prev,
      [name]: value
    }))
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-5xl font-bold mb-10 text-gray-800 dark:text-gray-100">Маршрутизация</h1>

      {/* Routing Policy Configuration */}
      <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl mb-8">
        <h2 className="text-3xl font-bold mb-6 dark:text-white">Конфигурация политики маршрутизации</h2>
        <form onSubmit={updateRoutingPolicy} className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Стратегия по умолчанию</label>
              <select
                name="default_strategy"
                value={policy.default_strategy}
                onChange={handlePolicyChange}
                className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              >
                <option value="round_robin">Round Robin</option>
                <option value="least_loaded">Least Loaded</option>
                <option value="geo_preferred">Geo Preferred</option>
                <option value="model_specific">Model Specific</option>
                <option value="hybrid">Hybrid</option>
              </select>
            </div>

            <div className="flex items-center space-x-4">
              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  name="enable_geo_routing"
                  checked={policy.enable_geo_routing}
                  onChange={handlePolicyChange}
                  className="h-5 w-5 text-blue-600"
                />
                <span className="text-gray-700 dark:text-gray-300">Включить географическую маршрутизацию</span>
              </label>

              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  name="enable_load_balancing"
                  checked={policy.enable_load_balancing}
                  onChange={handlePolicyChange}
                  className="h-5 w-5 text-blue-600"
                />
                <span className="text-gray-700 dark:text-gray-300">Включить балансировку нагрузки</span>
              </label>

              <label className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  name="enable_model_specific"
                  checked={policy.enable_model_specific}
                  onChange={handlePolicyChange}
                  className="h-5 w-5 text-blue-600"
                />
                <span className="text-gray-700 dark:text-gray-300">Включить маршрутизацию по модели</span>
              </label>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Приоритет регионов (через запятую)</label>
              <input
                type="text"
                name="region_priority"
                value={policy.strategy_config?.region_priority || ''}
                onChange={(e) => {
                  setPolicy(prev => ({
                    ...prev,
                    strategy_config: {
                      ...prev.strategy_config,
                      region_priority: e.target.value
                    }
                  }))
                }}
                placeholder="e.g., us,eu,ru,cn,br"
                className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Конфигурация стратегии (JSON)</label>
              <textarea
                name="strategy_config"
                value={JSON.stringify(policy.strategy_config, null, 2)}
                onChange={(e) => {
                  try {
                    setPolicy(prev => ({
                      ...prev,
                      strategy_config: JSON.parse(e.target.value)
                    }))
                  } catch {
                    // Invalid JSON, don't update
                  }
                }}
                className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                rows="4"
              />
            </div>
          </div>

          <button
            type="submit"
            className="bg-blue-600 text-white px-6 py-2 rounded-md hover:bg-blue-700 transition"
          >
            Обновить политику
          </button>
        </form>
      </div>

      {/* Head Services */}
      <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl mb-8">
        <h2 className="text-3xl font-bold mb-6 dark:text-white">Сервисы Head</h2>

        {/* Region filter */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Фильтр по региону</label>
          <select
            onChange={(e) => setRegionFilter(e.target.value)}
            className="w-full max-w-xs p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
          >
            <option value="">Все регионы</option>
            {REGIONS.map(region => (
              <option key={region.code} value={region.code}>
                {region.name} ({region.code})
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredHeads.map(head => (
            <div key={head.head_id} className="bg-gray-50 dark:bg-gray-700 p-6 rounded-xl shadow-md">
              {/* Show region prominently with flag */}
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-xl font-semibold dark:text-white">
                  {head.head_id}
                  {head.region && (
                    <span className="ml-2 px-2 py-1 text-xs font-bold text-white bg-blue-500 rounded-full">
                      {head.region.toUpperCase()}
                    </span>
                  )}
                </h3>
                <span className={`text-sm ${head.status === 'active' ? 'text-green-500' : head.status === 'draining' ? 'text-yellow-500' : 'text-red-500'}`}>
                  ● {head.status}
                </span>
              </div>

              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Endpoint:</strong> {head.endpoint}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Нагрузка:</strong> {head.current_load}%</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Тип модели:</strong> {head.model_type}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Версия:</strong> {head.version}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Последний heartbeat:</strong> {new Date(head.last_heartbeat * 1000).toLocaleString()}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Register New Head */}
      <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl">
        <h2 className="text-3xl font-bold mb-6 dark:text-white">Регистрация нового Head</h2>
        <form onSubmit={registerHead} className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Head ID</label>
            <input
              type="text"
              name="head_id"
              value={newHead.head_id}
              onChange={(e) => {
                handleNewHeadChange(e)
                // Auto-suggest region from head_id suffix
                const headId = e.target.value
                const regionMatch = headId.match(/-([a-z]{2})$/i)
                if (regionMatch && REGIONS.some(r => r.code.toLowerCase() === regionMatch[1].toLowerCase())) {
                  setNewHead(prev => ({
                    ...prev,
                    region: regionMatch[1].toLowerCase()
                  }))
                }
              }}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
            {newHead.head_id && (
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Пример: head-service-{REGIONS[0].code}, head-service-{REGIONS[1].code}
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Endpoint</label>
            <input
              type="text"
              name="endpoint"
              value={newHead.endpoint}
              onChange={handleNewHeadChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Регион</label>
            <select
              name="region"
              value={newHead.region}
              onChange={handleNewHeadChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            >
              <option value="">Выберите регион</option>
              {REGIONS.map(region => (
                <option key={region.code} value={region.code}>
                  {region.name} ({region.code})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Тип модели</label>
            <input
              type="text"
              name="model_type"
              value={newHead.model_type}
              onChange={handleNewHeadChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Версия</label>
            <input
              type="text"
              name="version"
              value={newHead.version}
              onChange={handleNewHeadChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Метаданные (JSON)</label>
            <textarea
              name="metadata"
              value={JSON.stringify(newHead.metadata, null, 2)}
              onChange={(e) => {
                try {
                  setNewHead(prev => ({
                    ...prev,
                    metadata: JSON.parse(e.target.value)
                  }))
                } catch {
                  // Invalid JSON, don't update
                }
              }}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              rows="3"
            />
          </div>

          <div className="md:col-span-2">
            <button
              type="submit"
              className="bg-green-600 text-white px-6 py-2 rounded-md hover:bg-green-700 transition"
            >
              Зарегистрировать Head
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
