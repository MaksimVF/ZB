


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

// Model management endpoints
app.get('/admin/models', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // For now, return a mock response
    // In a real implementation, this would fetch from a database or service
    const models = [
      {
        model_id: 'mistral-7b',
        name: 'Mistral 7B',
        provider: 'mistral',
        region: 'eu',
        endpoint: 'https://api.mistral.eu/v1/models/mistral-7b',
        priority_head: 'head-service-eu',
        local_inference: false,
        api_key: '***',
        metadata: {}
      },
      {
        model_id: 'kimi-13b',
        name: 'Kimi 13B',
        provider: 'kimi',
        region: 'cn',
        endpoint: 'https://api.kimi.cn/v1/models/kimi-13b',
        priority_head: 'head-service-cn',
        local_inference: false,
        api_key: '***',
        metadata: {}
      },
      {
        model_id: 'gemini-pro',
        name: 'Gemini Pro',
        provider: 'google',
        region: 'us',
        endpoint: 'https://api.gemini.us/v1/models/gemini-pro',
        priority_head: 'head-service-us',
        local_inference: false,
        api_key: '***',
        metadata: {}
      }
    ]

    res.json(models);
  } catch (error) {
    console.error('Error fetching models:', error);
    res.status(500).json({ error: 'Failed to fetch models' });
  }
});

app.post('/admin/models', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // For now, return a mock response
    // In a real implementation, this would save to a database or service
    console.log('Model created:', req.body);
    res.json({ success: true, model: req.body });
  } catch (error) {
    console.error('Error creating model:', error);
    res.status(500).json({ error: 'Failed to create model' });
  }
});

app.put('/admin/models/:model_id', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // For now, return a mock response
    // In a real implementation, this would update a database or service
    console.log('Model updated:', req.params.model_id, req.body);
    res.json({ success: true, model: req.body });
  } catch (error) {
    console.error('Error updating model:', error);
    res.status(500).json({ error: 'Failed to update model' });
  }
});

app.delete('/admin/models/:model_id', async (req, res) => {
  try {
    const adminKey = req.headers['x-admin-key'];
    if (adminKey !== process.env.ADMIN_KEY) {
      return res.status(403).json({ error: 'Forbidden' });
    }

    // For now, return a mock response
    // In a real implementation, this would delete from a database or service
    console.log('Model deleted:', req.params.model_id);
    res.json({ success: true });
  } catch (error) {
    console.error('Error deleting model:', error);
    res.status(500).json({ error: 'Failed to delete model' });
  }
});

app.listen(port, () => {
  console.log(`Admin UI server running at http://localhost:${port}`);
});


