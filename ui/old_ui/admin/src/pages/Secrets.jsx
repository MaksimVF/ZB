// ui/admin/src/tabs/Secrets.jsx
export default function Secrets() {
  const [secrets, setSecrets] = useState({})

  const load = () => fetch("http://localhost:8081/admin/api/secrets", {
    headers: { "X-Admin-Key": "devkey123" }
  }).then(r => r.json()).then(setSecrets)

  useEffect(() => { load(); const i = setInterval(load, 10000); return () => clearInterval(i) }, [])

  const save = (provider) => {
    const apiKey = prompt("Введите API-ключ для " + provider)
    if (!apiKey) return

    fetch("http://localhost:8081/admin/api/secrets", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Admin-Key": "devkey123" },
      body: JSON.stringify({
        provider,
        secrets: { api_key: apiKey }
      })
    }).then(() => load())
  }

  return (
    <div className="space-y-4">
      <h2 className="text-2xl">Секреты и API-ключи</h2>
      {Object.entries(secrets).map(([prov, data]) => (
        <div key={prov} className="border p-4 rounded">
          <strong>{prov}</strong><br/>
          API Key: {data.api_key || "не задан"}<br/>
          Updated: {new Date(data.updated_at * 1000).toLocaleString()}
        </div>
      ))}
      <div className="flex gap-2">
        <button onClick={() => save("openai")} className="bg-blue-600 text-white px-4 py-2 rounded">OpenAI</button>
        <button onClick={() => save("anthropic")}>Anthropic</button>
        <button onClick={() => save("groq")}>Groq</button>
        <button onClick={() => save("together")}>Together</button>
      </div>
    </div>
  )
}
