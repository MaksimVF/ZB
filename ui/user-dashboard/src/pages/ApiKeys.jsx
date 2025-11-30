











import { useState, useEffect } from 'react'
import { api } from '../api'

export default function ApiKeys() {
  const [apiKeys, setApiKeys] = useState([])
  const [loading, setLoading] = useState(true)
  const [newKeyName, setNewKeyName] = useState('')

  useEffect(() => {
    const load = async () => {
      const res = await api.getApiKeys()
      setApiKeys(res.data)
      setLoading(false)
    }
    load()
  }, [])

  const createKey = async () => {
    if (!newKeyName) return
    const res = await api.createApiKey({ name: newKeyName })
    setApiKeys([...apiKeys, res.data])
    setNewKeyName('')
  }

  const deleteKey = async (id) => {
    await api.deleteApiKey(id)
    setApiKeys(apiKeys.filter(k => k.id !== id))
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-4xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">API Keys</h1>

      <div className="bg-white p-6 rounded-xl shadow-lg mb-8">
        <h2 className="text-2xl font-bold mb-4">Create New API Key</h2>
        <div className="flex gap-4">
          <input
            type="text"
            placeholder="Key name"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            className="flex-1 p-3 border border-gray-300 rounded-lg"
          />
          <button
            onClick={createKey}
            className="bg-indigo-600 text-white px-6 py-3 rounded-lg hover:bg-indigo-700"
          >
            Create
          </button>
        </div>
      </div>

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
                    <code className="bg-gray-100 p-2 rounded">{key.key}</code>
                  </td>
                  <td className="p-4">{new Date(key.created_at).toLocaleString()}</td>
                  <td className="p-4">
                    <button
                      onClick={() => deleteKey(key.id)}
                      className="bg-red-600 text-white px-4 py-2 rounded-lg hover:bg-red-700"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}











