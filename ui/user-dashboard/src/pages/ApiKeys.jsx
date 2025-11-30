











import { useState, useEffect } from 'react'
import { apiKeys } from '../api'

export default function ApiKeys() {
  const [apiKeys, setApiKeys] = useState([])
  const [loading, setLoading] = useState(true)
  const [newKeyName, setNewKeyName] = useState('')
  const [newKeyPermissions, setNewKeyPermissions] = useState({
    read: true,
    write: false,
    delete: false
  })
  const [selectedKey, setSelectedKey] = useState(null)
  const [usageData, setUsageData] = useState(null)

  useEffect(() => {
    const load = async () => {
      const res = await apiKeys.getApiKeys()
      setApiKeys(res.data)
      setLoading(false)
    }
    load()
  }, [])

  const createKey = async () => {
    if (!newKeyName) return
    const res = await apiKeys.createApiKey({
      name: newKeyName,
      permissions: newKeyPermissions
    })
    setApiKeys([...apiKeys, res.data])
    setNewKeyName('')
    setNewKeyPermissions({ read: true, write: false, delete: false })
  }

  const deleteKey = async (id) => {
    await apiKeys.deleteApiKey(id)
    setApiKeys(apiKeys.filter(k => k.id !== id))
  }

  const rotateKey = async (id) => {
    const res = await apiKeys.rotateApiKey(id)
    setApiKeys(apiKeys.map(k => k.id === id ? { ...k, key: res.data.key } : k))
  }

  const loadUsage = async (id) => {
    const res = await apiKeys.getApiKeyUsage(id)
    setUsageData(res.data)
    setSelectedKey(id)
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-6xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">API Keys</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Create Key Section */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Create New API Key</h2>

          <div className="mb-4">
            <label className="block text-gray-700 mb-1">Key Name</label>
            <input
              type="text"
              placeholder="Key name"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              className="w-full p-3 border border-gray-300 rounded-lg"
            />
          </div>

          <div className="mb-4">
            <h3 className="text-lg font-bold mb-2">Permissions</h3>
            <div className="space-y-2">
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={newKeyPermissions.read}
                  onChange={(e) => setNewKeyPermissions({ ...newKeyPermissions, read: e.target.checked })}
                  className="mr-2"
                />
                Read access
              </label>
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={newKeyPermissions.write}
                  onChange={(e) => setNewKeyPermissions({ ...newKeyPermissions, write: e.target.checked })}
                  className="mr-2"
                />
                Write access
              </label>
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={newKeyPermissions.delete}
                  onChange={(e) => setNewKeyPermissions({ ...newKeyPermissions, delete: e.target.checked })}
                  className="mr-2"
                />
                Delete access
              </label>
            </div>
          </div>

          <button
            onClick={createKey}
            className="bg-indigo-600 text-white px-6 py-3 rounded-lg hover:bg-indigo-700"
          >
            Create API Key
          </button>
        </div>

        {/* API Keys List */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Your API Keys</h2>

          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-900 text-white">
                <tr>
                  <th className="p-4 text-left">Name</th>
                  <th className="p-4 text-left">Key</th>
                  <th className="p-4 text-left">Created</th>
                  <th className="p-4 text-left">Actions</th>
                </tr>
              </thead>
              <tbody>
                {apiKeys.map(key => (
                  <tr key={key.id} className="border-t hover:bg-gray-50">
                    <td className="p-4">{key.name}</td>
                    <td className="p-4">
                      <code className="bg-gray-100 p-2 rounded text-sm break-all">{key.key}</code>
                    </td>
                    <td className="p-4">{new Date(key.created_at).toLocaleString()}</td>
                    <td className="p-4">
                      <div className="flex space-x-2">
                        <button
                          onClick={() => loadUsage(key.id)}
                          className="bg-blue-600 text-white px-3 py-1 rounded-lg hover:bg-blue-700 text-sm"
                        >
                          Usage
                        </button>
                        <button
                          onClick={() => rotateKey(key.id)}
                          className="bg-yellow-600 text-white px-3 py-1 rounded-lg hover:bg-yellow-700 text-sm"
                        >
                          Rotate
                        </button>
                        <button
                          onClick={() => deleteKey(key.id)}
                          className="bg-red-600 text-white px-3 py-1 rounded-lg hover:bg-red-700 text-sm"
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Usage Data */}
      {selectedKey && usageData && (
        <div className="mt-8 bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Usage for {apiKeys.find(k => k.id === selectedKey)?.name}</h2>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div>
              <h3 className="font-bold mb-2">Total Requests</h3>
              <p className="text-3xl">{usageData.total_requests}</p>
            </div>
            <div>
              <h3 className="font-bold mb-2">Last 30 Days</h3>
              <p className="text-3xl">{usageData.last_30_days}</p>
            </div>
            <div>
              <h3 className="font-bold mb-2">Cost</h3>
              <p className="text-3xl">${usageData.total_cost.toFixed(2)}</p>
            </div>
          </div>

          <div className="mt-6">
            <h3 className="font-bold mb-2">Permissions</h3>
            <div className="flex space-x-4">
              {usageData.permissions.read && <span className="bg-green-100 text-green-800 px-3 py-1 rounded-full text-sm">Read</span>}
              {usageData.permissions.write && <span className="bg-blue-100 text-blue-800 px-3 py-1 rounded-full text-sm">Write</span>}
              {usageData.permissions.delete && <span className="bg-red-100 text-red-800 px-3 py-1 rounded-full text-sm">Delete</span>}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}











