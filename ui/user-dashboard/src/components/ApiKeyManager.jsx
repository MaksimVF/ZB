




import React, { useState, useEffect } from 'react';
import axios from 'axios';

const ApiKeyManager = ({ userId }) => {
  const [apiKeys, setApiKeys] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [newKey, setNewKey] = useState({ provider: '', key: '' });

  useEffect(() => {
    const fetchApiKeys = async () => {
      try {
        const response = await axios.get('/api/user/secrets', {
          headers: { 'X-User-ID': userId }
        });
        setApiKeys(response.data);
        setLoading(false);
      } catch (err) {
        setError('Failed to fetch API keys');
        setLoading(false);
      }
    };

    fetchApiKeys();
  }, [userId]);

  const handleSave = async () => {
    try {
      await axios.post('/api/user/secrets', {
        secretName: `llm/${newKey.provider}/api_key`,
        secretValue: newKey.key
      }, {
        headers: { 'X-User-ID': userId }
      });

      // Refresh the list
      const response = await axios.get('/api/user/secrets', {
        headers: { 'X-User-ID': userId }
      });
      setApiKeys(response.data);
      setNewKey({ provider: '', key: '' });
    } catch (err) {
      setError('Failed to save API key');
    }
  };

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div className="api-key-manager">
      <h2>Manage Your API Keys</h2>

      <div className="current-keys">
        <h3>Your Current API Keys</h3>
        {Object.keys(apiKeys).length === 0 ? (
          <p>No API keys saved yet.</p>
        ) : (
          <ul>
            {Object.entries(apiKeys).map(([provider, key]) => (
              <li key={provider}>
                <strong>{provider}:</strong> {key}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="add-key-form">
        <h3>Add New API Key</h3>
        <div>
          <label>Provider:</label>
          <select
            value={newKey.provider}
            onChange={(e) => setNewKey({ ...newKey, provider: e.target.value })}
          >
            <option value="">Select provider</option>
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="google">Google</option>
            <option value="meta">Meta</option>
          </select>
        </div>

        <div>
          <label>API Key:</label>
          <input
            type="password"
            value={newKey.key}
            onChange={(e) => setNewKey({ ...newKey, key: e.target.value })}
            placeholder="Enter your API key"
          />
        </div>

        <button onClick={handleSave} disabled={!newKey.provider || !newKey.key}>
          Save API Key
        </button>
      </div>
    </div>
  );
};

export default ApiKeyManager;




