


const express = require('express');
const axios = require('axios');
const app = express();
const port = 5173;

app.use(express.static('dist'));
app.use(express.json());

// Admin API endpoints
app.get('/api/metrics', (req, res) => {
  res.json({
    totalRequests: 1000,
    activeUsers: 150,
    responseTime: '200ms'
  });
});

// Secrets management - proxy to secrets-service
app.get('/admin/api/secrets', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to secrets-service
    const response = await axios.get('http://secret-service:8082/admin/api/secrets', {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error fetching secrets:', error);
    res.status(500).json({ error: 'Failed to fetch secrets' });
  }
});

app.post('/admin/api/secrets', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to secrets-service
    const response = await axios.post('http://secret-service:8082/admin/api/secrets', req.body, {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error saving secret:', error);
    res.status(500).json({ error: 'Failed to save secret' });
  }
});

// Routing service API endpoints
app.get('/admin/routing/policy', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to routing-service
    const response = await axios.get('http://routing-service:8080/api/routing/policy', {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error fetching routing policy:', error);
    res.status(500).json({ error: 'Failed to fetch routing policy' });
  }
});

app.put('/admin/routing/policy', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to routing-service
    const response = await axios.put('http://routing-service:8080/api/routing/policy', req.body, {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error updating routing policy:', error);
    res.status(500).json({ error: 'Failed to update routing policy' });
  }
});

app.get('/admin/routing/heads', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to routing-service
    const response = await axios.get('http://routing-service:8080/api/routing/heads', {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error fetching head services:', error);
    res.status(500).json({ error: 'Failed to fetch head services' });
  }
});

app.post('/admin/routing/heads', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // Forward request to routing-service
    const response = await axios.post('http://routing-service:8080/api/routing/heads', req.body, {
      headers: { 'X-Admin-Key': adminKey }
    });
    res.json(response.data);
  } catch (error) {
    console.error('Error registering head:', error);
    res.status(500).json({ error: 'Failed to register head' });
  }
});

app.listen(port, () => {
  console.log(`Admin UI server running at http://localhost:${port}`);
});


