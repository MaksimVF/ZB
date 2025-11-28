


const express = require('express');
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

app.listen(port, () => {
  console.log(`Admin UI server running at http://localhost:${port}`);
});


