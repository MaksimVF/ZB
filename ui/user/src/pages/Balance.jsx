export default function Balance() {
  const [balance, setBalance] = useState(0)
  const [usage, setUsage] = useState([])

  return (
    <div className="max-w-4xl mx-auto p-8">
      <h1 className="text-4xl mb-8">Ваш баланс</h1>
     
      <div className="bg-gradient-to-r from-blue-500 to-purple-600 text-white p-10 rounded-2xl">
        <div className="text-6xl font-bold">${balance.toFixed(2)}</div>
        <div className="text-xl opacity-90">осталось на счету</div>
      </div>

      <div className="mt-8">
        <a href="/topup" className="bg-green-600 text-white px-8 py-4 rounded text-xl">
          Пополнить счёт
        </a>
      </div>

      <h2 className="text-2xl mt-12">Использование за месяц</h2>
      <BarChart data={usage} />
    </div>
  )
}
