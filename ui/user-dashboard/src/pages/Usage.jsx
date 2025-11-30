








import { useEffect, useState } from 'react'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { billing } from '../api'

export default function Usage() {
  const [usage, setUsage] = useState([])

  useEffect(() => {
    const load = async () => {
      const usg = await billing.usage()
      setUsage(usg.data.last_30_days)
    }
    load()
  }, [])

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Использование</h1>

      <div className="bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Расходы за 30 дней</h2>
        <ResponsiveContainer width="100%" height={400}>
          <LineChart data={usage}>
            <XAxis dataKey="date" />
            <YAxis />
            <Tooltip formatter={(v) => `$${v.toFixed(2)}`} />
            <Line type="monotone" dataKey="cost" stroke="#8b5cf6" strokeWidth={3} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}









