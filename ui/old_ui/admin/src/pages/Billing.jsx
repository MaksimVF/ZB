// ui/admin/src/tabs/Billing.jsx
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'

export default function Billing() {
  const [users, setUsers] = useState([])
  const [totalSpent, setTotalSpent] = useState(0)

  useEffect(() => {
    fetch("/admin/api/billing/stats").then(r => r.json()).then(data => {
      setUsers(data.users)
      setTotalSpent(data.total_cents / 100)
    })
  }, [])

  return (
    <div className="space-y-8">
      <h2 className="text-3xl">Биллинг и расходы</h2>
     
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-blue-100 p-6 rounded">Всего заработано: ${totalSpent.toFixed(2)}</div>
        <div className="bg-green-100 p-6 rounded">Активных пользователей: {users.length}</div>
        <div className="bg-purple-100 p-6 rounded">Сегодня: $124.51</div>
      </div>

      <h3>Расходы по пользователям (последние 7 дней)</h3>
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={users}>
          <XAxis dataKey="date" />
          <YAxis />
          <Tooltip formatter={(v) => `$${v}`} />
          {users.map(u => <Line key={u.id} dataKey={u.id} name={u.name} stroke={u.color} />)}
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
