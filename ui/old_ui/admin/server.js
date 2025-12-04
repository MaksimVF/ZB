


const express = require('express');
const axios = require('axios');
const app = express();
const port = 59272;

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

app.get('/api/billing', (req, res) => {
  res.json({
    totalRevenue: '$5000',
    pendingPayments: '$250',
    nextBillingCycle: '2025-12-01'
  });
});

app.get('/api/queues', (req, res) => {
  res.json({
    activeQueues: 3,
    pendingJobs: 12,
    completedJobs: 450
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

app.listen(port, () => {
  console.log(`Admin UI server running at http://localhost:${port}`);
});


