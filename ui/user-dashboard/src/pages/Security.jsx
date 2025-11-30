












import { useState, useEffect } from 'react'
import { auth } from '../api'

export default function Security() {
  const [twoFactorEnabled, setTwoFactorEnabled] = useState(false)
  const [qrCode, setQrCode] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sessions, setSessions] = useState([])
  const [activityLogs, setActivityLogs] = useState([])

  useEffect(() => {
    const load = async () => {
      try {
        const res = await auth.get2FAStatus()
        setTwoFactorEnabled(res.data.enabled)

        // Load sessions
        const sessionsRes = await auth.getSessions()
        setSessions(sessionsRes.data)

        // Load activity logs
        const logsRes = await auth.getActivityLogs()
        setActivityLogs(logsRes.data)
      } catch (e) {
        setError('Failed to load security data')
      }
      setLoading(false)
    }
    load()
  }, [])

  const enable2FA = async () => {
    setLoading(true)
    try {
      const res = await auth.enable2FA()
      setQrCode(res.data.qr_code_url)
    } catch (e) {
      setError('Failed to enable 2FA')
    }
    setLoading(false)
  }

  const verify2FA = async () => {
    setLoading(true)
    try {
      await auth.verify2FA({ code: verificationCode })
      setTwoFactorEnabled(true)
      setQrCode('')
      setVerificationCode('')
    } catch (e) {
      setError('Invalid verification code')
    }
    setLoading(false)
  }

  const disable2FA = async () => {
    setLoading(true)
    try {
      await auth.disable2FA()
      setTwoFactorEnabled(false)
    } catch (e) {
      setError('Failed to disable 2FA')
    }
    setLoading(false)
  }

  const terminateSession = async (sessionId) => {
    setLoading(true)
    try {
      await auth.terminateSession(sessionId)
      setSessions(sessions.filter(s => s.id !== sessionId))
    } catch (e) {
      setError('Failed to terminate session')
    }
    setLoading(false)
  }

  if (loading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-6xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Security Settings</h1>

      {/* 2FA Section */}
      <div className="bg-white p-6 rounded-xl shadow-lg mb-8">
        <h2 className="text-2xl font-bold mb-4">Two-Factor Authentication</h2>

        {error && <div className="mb-4 p-3 bg-red-100 text-red-700 rounded">{error}</div>}

        {twoFactorEnabled ? (
          <div>
            <p className="mb-4">Two-factor authentication is currently <strong>enabled</strong>.</p>
            <button
              onClick={disable2FA}
              disabled={loading}
              className="bg-red-600 text-white px-6 py-3 rounded-lg hover:bg-red-700 disabled:opacity-50"
            >
              Disable 2FA
            </button>
          </div>
        ) : (
          <div>
            {qrCode ? (
              <div>
                <p className="mb-4">Scan this QR code with your authenticator app:</p>
                <img src={qrCode} alt="QR Code" className="mb-4" />
                <div className="mb-4">
                  <input
                    type="text"
                    placeholder="Enter verification code"
                    value={verificationCode}
                    onChange={(e) => setVerificationCode(e.target.value)}
                    className="p-3 border border-gray-300 rounded-lg w-full max-w-sm"
                  />
                </div>
                <button
                  onClick={verify2FA}
                  disabled={loading}
                  className="bg-indigo-600 text-white px-6 py-3 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                >
                  Verify and Enable 2FA
                </button>
              </div>
            ) : (
              <div>
                <p className="mb-4">Two-factor authentication is currently <strong>disabled</strong>.</p>
                <button
                  onClick={enable2FA}
                  disabled={loading}
                  className="bg-indigo-600 text-white px-6 py-3 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                >
                  Enable 2FA
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Active Sessions */}
      <div className="bg-white p-6 rounded-xl shadow-lg mb-8">
        <h2 className="text-2xl font-bold mb-4">Active Sessions</h2>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-100">
              <tr>
                <th className="p-4 text-left">Device</th>
                <th className="p-4 text-left">Location</th>
                <th className="p-4 text-left">Last Active</th>
                <th className="p-4 text-left">Actions</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map(session => (
                <tr key={session.id} className="border-t hover:bg-gray-50">
                  <td className="p-4">{session.device_type} - {session.browser}</td>
                  <td className="p-4">{session.city}, {session.country}</td>
                  <td className="p-4">{new Date(session.last_active).toLocaleString()}</td>
                  <td className="p-4">
                    {session.current ? (
                      <span className="bg-green-100 text-green-800 px-3 py-1 rounded-full text-sm">Current</span>
                    ) : (
                      <button
                        onClick={() => terminateSession(session.id)}
                        className="bg-red-600 text-white px-3 py-1 rounded-lg hover:bg-red-700 text-sm"
                      >
                        Terminate
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Activity Logs */}
      <div className="bg-white p-6 rounded-xl shadow-lg">
        <h2 className="text-2xl font-bold mb-4">Activity Logs</h2>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-100">
              <tr>
                <th className="p-4 text-left">Date</th>
                <th className="p-4 text-left">Action</th>
                <th className="p-4 text-left">IP Address</th>
                <th className="p-4 text-left">Status</th>
              </tr>
            </thead>
            <tbody>
              {activityLogs.map(log => (
                <tr key={log.id} className="border-t hover:bg-gray-50">
                  <td className="p-4">{new Date(log.timestamp).toLocaleString()}</td>
                  <td className="p-4">{log.action}</td>
                  <td className="p-4">{log.ip_address}</td>
                  <td className="p-4">
                    <span className={`px-3 py-1 rounded-full text-white text-sm ${log.status === 'success' ? 'bg-green-600' : 'bg-red-600'}`}>
                      {log.status}
                    </span>
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












