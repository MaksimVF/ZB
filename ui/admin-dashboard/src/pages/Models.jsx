


import { useState, useEffect } from 'react'
import { admin } from '../api'

export default function Models() {
  const [models, setModels] = useState([])
  const [heads, setHeads] = useState([])
  const [editingModel, setEditingModel] = useState(null)
  const [newModel, setNewModel] = useState({
    model_id: '',
    name: '',
    provider: '',
    region: '',
    endpoint: '',
    priority_head: '',
    local_inference: false,
    api_key: '',
    metadata: {}
  })

  useEffect(() => {
    loadModels()
    loadHeadServices()
  }, [])

  const loadModels = async () => {
    try {
      const response = await admin.models()
      setModels(response.data)
    } catch (error) {
      console.error('Failed to load models:', error)
    }
  }

  const loadHeadServices = async () => {
    try {
      const response = await admin.headServices()
      setHeads(response.data)
    } catch (error) {
      console.error('Failed to load head services:', error)
    }
  }

  const handleEdit = (model) => {
    setEditingModel({ ...model })
  }

  const handleSave = async () => {
    try {
      if (editingModel.model_id) {
        await admin.updateModel(editingModel)
      } else {
        await admin.createModel(editingModel)
      }
      setEditingModel(null)
      loadModels()
    } catch (error) {
      console.error('Failed to save model:', error)
    }
  }

  const handleDelete = async (model_id) => {
    if (confirm('Are you sure you want to delete this model?')) {
      try {
        await admin.deleteModel(model_id)
        loadModels()
      } catch (error) {
        console.error('Failed to delete model:', error)
      }
    }
  }

  const handleNewModelChange = (e) => {
    const { name, value, type, checked } = e.target
    setNewModel(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value
    }))
  }

  const handleEditModelChange = (e) => {
    const { name, value, type, checked } = e.target
    setEditingModel(prev => ({
      ...prev,
      [name]: type === 'checkbox' ? checked : value
    }))
  }

  const createModel = async (e) => {
    e.preventDefault()
    try {
      await admin.createModel(newModel)
      setNewModel({
        model_id: '',
        name: '',
        provider: '',
        region: '',
        endpoint: '',
        priority_head: '',
        local_inference: false,
        api_key: '',
        metadata: {}
      })
      loadModels()
    } catch (error) {
      console.error('Failed to create model:', error)
    }
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-5xl font-bold mb-10 text-gray-800 dark:text-gray-100">Управление моделями</h1>

      {/* Model List */}
      <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl mb-8">
        <h2 className="text-3xl font-bold mb-6 dark:text-white">Список моделей</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {models.map(model => (
            <div key={model.model_id} className="bg-gray-50 dark:bg-gray-700 p-6 rounded-xl shadow-md">
              <h3 className="text-xl font-semibold mb-3 dark:text-white">{model.name}</h3>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>ID:</strong> {model.model_id}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Провайдер:</strong> {model.provider}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Регион:</strong> {model.region}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Приоритетный Head:</strong> {model.priority_head}</p>
              <p className="text-gray-600 dark:text-gray-300 mb-1"><strong>Локальный инференс:</strong> {model.local_inference ? 'Да' : 'Нет'}</p>
              <div className="mt-4 space-x-2">
                <button
                  onClick={() => handleEdit(model)}
                  className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600"
                >
                  Редактировать
                </button>
                <button
                  onClick={() => handleDelete(model.model_id)}
                  className="px-4 py-2 bg-red-500 text-white rounded-md hover:bg-red-600"
                >
                  Удалить
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Create New Model */}
      <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl mb-8">
        <h2 className="text-3xl font-bold mb-6 dark:text-white">Создать новую модель</h2>
        <form onSubmit={createModel} className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Model ID</label>
            <input
              type="text"
              name="model_id"
              value={newModel.model_id}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Название</label>
            <input
              type="text"
              name="name"
              value={newModel.name}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Провайдер</label>
            <input
              type="text"
              name="provider"
              value={newModel.provider}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Регион</label>
            <select
              name="region"
              value={newModel.region}
              onChange={handleNewModelChange}
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
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Endpoint</label>
            <input
              type="text"
              name="endpoint"
              value={newModel.endpoint}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Приоритетный Head</label>
            <select
              name="priority_head"
              value={newModel.priority_head}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
            >
              <option value="">Выберите Head</option>
              {heads.map(head => (
                <option key={head.head_id} value={head.head_id}>
                  {head.head_id} ({head.region})
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center space-x-2">
            <input
              type="checkbox"
              name="local_inference"
              checked={newModel.local_inference}
              onChange={handleNewModelChange}
              className="h-5 w-5 text-blue-600"
            />
            <label className="text-gray-700 dark:text-gray-300">Включить локальный инференс</label>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Key</label>
            <input
              type="password"
              name="api_key"
              value={newModel.api_key}
              onChange={handleNewModelChange}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Метаданные (JSON)</label>
            <textarea
              name="metadata"
              value={JSON.stringify(newModel.metadata, null, 2)}
              onChange={(e) => {
                try {
                  setNewModel(prev => ({
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
              className="w-full bg-green-500 text-white p-4 rounded-md hover:bg-green-600"
            >
              Создать модель
            </button>
          </div>
        </form>
      </div>

      {/* Edit Model Modal */}
      {editingModel && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4">
          <div className="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl w-full max-w-2xl">
            <h2 className="text-3xl font-bold mb-6 dark:text-white">Редактировать модель</h2>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                handleSave()
              }}
              className="grid grid-cols-1 md:grid-cols-2 gap-6"
            >
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Model ID</label>
                <input
                  type="text"
                  name="model_id"
                  value={editingModel.model_id}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                  disabled
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Название</label>
                <input
                  type="text"
                  name="name"
                  value={editingModel.name}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Провайдер</label>
                <input
                  type="text"
                  name="provider"
                  value={editingModel.provider}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Регион</label>
                <select
                  name="region"
                  value={editingModel.region}
                  onChange={handleEditModelChange}
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
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Endpoint</label>
                <input
                  type="text"
                  name="endpoint"
                  value={editingModel.endpoint}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Приоритетный Head</label>
                <select
                  name="priority_head"
                  value={editingModel.priority_head}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                >
                  <option value="">Выберите Head</option>
                  {heads.map(head => (
                    <option key={head.head_id} value={head.head_id}>
                      {head.head_id} ({head.region})
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  name="local_inference"
                  checked={editingModel.local_inference}
                  onChange={handleEditModelChange}
                  className="h-5 w-5 text-blue-600"
                />
                <label className="text-gray-700 dark:text-gray-300">Включить локальный инференс</label>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Key</label>
                <input
                  type="password"
                  name="api_key"
                  value={editingModel.api_key}
                  onChange={handleEditModelChange}
                  className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-md dark:bg-gray-700 dark:text-white"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Метаданные (JSON)</label>
                <textarea
                  name="metadata"
                  value={JSON.stringify(editingModel.metadata, null, 2)}
                  onChange={(e) => {
                    try {
                      setEditingModel(prev => ({
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

              <div className="md:col-span-2 flex space-x-4">
                <button
                  type="submit"
                  className="flex-1 bg-green-500 text-white p-4 rounded-md hover:bg-green-600"
                >
                  Сохранить
                </button>
                <button
                  type="button"
                  onClick={() => setEditingModel(null)}
                  className="flex-1 bg-gray-300 text-gray-700 p-4 rounded-md hover:bg-gray-400"
                >
                  Отмена
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

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


