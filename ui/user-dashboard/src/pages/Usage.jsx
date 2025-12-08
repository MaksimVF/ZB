








import { useState, useEffect } from 'react'
import { billing } from '../api'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, BarChart, Bar } from 'recharts'
import { CSVLink } from 'react-csv'

export default function Usage() {
  const [usageData, setUsageData] = useState([])
  const [loading, setLoading] = useState(true)
  const [timePeriod, setTimePeriod] = useState('30d')
  const [modelFilter, setModelFilter] = useState('all')
  const [models, setModels] = useState([])
  const [comparisonMode, setComparisonMode] = useState(false)
  const [comparisonPeriod, setComparisonPeriod] = useState('previous_month')

  useEffect(() => {
    const load = async () => {
      const res = await billing.usage({ period: timePeriod, model: modelFilter })
      setUsageData(res.data)
      setModels(res.data.available_models)
      setLoading(false)
    }
    load()
  }, [timePeriod, modelFilter])

  const handleExport = () => {
    const dataToExport = usageData.daily_usage.map(item => ({
      date: item.date,
      requests: item.requests,
      tokens: item.tokens,
      model: modelFilter === 'all' ? 'All Models' : modelFilter
    }))
    return dataToExport
  }

  const toggleComparison = () => {
    setComparisonMode(!comparisonMode)
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-6xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Usage Analytics</h1>

      {/* Filters and Controls */}
      <div className="mb-8 bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Filters</h2>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div>
            <label className="block text-gray-700 mb-2">Time Period</label>
            <select
              value={timePeriod}
              onChange={(e) => setTimePeriod(e.target.value)}
              className="w-full p-3 border border-gray-300 rounded-lg"
            >
              <option value="7d">Last 7 days</option>
              <option value="30d">Last 30 days</option>
              <option value="90d">Last 90 days</option>
              <option value="this_month">This month</option>
              <option value="last_month">Last month</option>
              <option value="all">All time</option>
            </select>
          </div>

          <div>
            <label className="block text-gray-700 mb-2">Model</label>
            <select
              value={modelFilter}
              onChange={(e) => setModelFilter(e.target.value)}
              className="w-full p-3 border border-gray-300 rounded-lg"
            >
              <option value="all">All Models</option>
              {models.map(model => (
                <option key={model.id} value={model.id}>{model.name}</option>
              ))}
            </select>
          </div>

          <div className="flex items-end space-x-4">
            <CSVLink
              data={handleExport()}
              filename={`usage_${timePeriod}_${modelFilter}.csv`}
              className="bg-green-600 text-white px-6 py-3 rounded-lg hover:bg-green-700"
            >
              Export CSV
            </CSVLink>

            <button
              onClick={toggleComparison}
              className={`px-6 py-3 rounded-lg ${comparisonMode ? 'bg-indigo-600 text-white' : 'bg-gray-200'}`}
            >
              {comparisonMode ? 'Disable Comparison' : 'Enable Comparison'}
            </button>
          </div>
        </div>

        {comparisonMode && (
          <div className="mt-6">
            <label className="block text-gray-700 mb-2">Compare With</label>
            <select
              value={comparisonPeriod}
              onChange={(e) => setComparisonPeriod(e.target.value)}
              className="w-full p-3 border border-gray-300 rounded-lg"
            >
              <option value="previous_month">Previous month</option>
              <option value="previous_30d">Previous 30 days</option>
              <option value="same_period_last_year">Same period last year</option>
            </select>
          </div>
        )}
      </div>

      {/* Analytics Charts */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {/* Daily Usage */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Daily Usage</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={usageData.daily_usage}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="requests" name="Requests" stroke="#8884d8" />
              <Line type="monotone" dataKey="tokens" name="Tokens" stroke="#82ca9d" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Model Usage */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Model Usage</h2>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={usageData.model_usage}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="model" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Bar dataKey="requests" name="Requests" fill="#8884d8" />
              <Bar dataKey="tokens" name="Tokens" fill="#82ca9d" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Comparison Chart */}
      {comparisonMode && (
        <div className="mt-8 bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Comparison: {timePeriod} vs {comparisonPeriod}</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={usageData.comparison_data}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="current_requests" name={`Current (${timePeriod})`} stroke="#8884d8" />
              <Line type="monotone" dataKey="comparison_requests" name={`Comparison (${comparisonPeriod})`} stroke="#ff7300" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Summary Statistics */}
      <div className="mt-8 grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h3 className="text-xl font-bold mb-2">Total Requests</h3>
          <p className="text-3xl">{usageData.total_requests}</p>
        </div>
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h3 className="text-xl font-bold mb-2">Total Tokens</h3>
          <p className="text-3xl">{usageData.total_tokens.toLocaleString()}</p>
        </div>
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h3 className="text-xl font-bold mb-2">Estimated Cost</h3>
          <p className="text-3xl">${usageData.estimated_cost.toFixed(2)}</p>
        </div>
      </div>
    </div>
  )
}









