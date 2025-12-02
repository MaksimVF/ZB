



const express = require('express');
const axios = require('axios');
const app = express();
const port = 5174;

app.use(express.static('dist'));
app.use(express.json());

// User API endpoints
app.get('/api/user/secrets', async (req, res) => {
  try {
    const userId = req.headers['x-user-id'];
    if (!userId) {
      return res.status(400).json({ error: 'User ID is required' });
    }

    // Forward request to secrets-service
    const response = await axios.get('http://secret-service:8082/api/user/secrets', {
      headers: { 'X-User-ID': userId }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error fetching user secrets:', error);
    res.status(500).json({ error: 'Failed to fetch user secrets' });
  }
});

app.post('/api/user/secrets', async (req, res) => {
  try {
    const userId = req.headers['x-user-id'];
    if (!userId) {
      return res.status(400).json({ error: 'User ID is required' });
    }

    const { secretName, secretValue } = req.body;
    if (!secretName || !secretValue) {
      return res.status(400).json({ error: 'Secret name and value are required' });
    }

    // Forward request to secrets-service
    const response = await axios.post('http://secret-service:8082/api/user/secrets', {
      secretName,
      secretValue
    }, {
      headers: { 'X-User-ID': userId }
    });

    res.json(response.data);
  } catch (error) {
    console.error('Error saving user secret:', error);
    res.status(500).json({ error: 'Failed to save user secret' });
  }
});

app.listen(port, () => {
  console.log(`User dashboard server running at http://localhost:${port}`);
});



