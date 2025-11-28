

const express = require('express');
const app = express();
const port = 54455;

app.use(express.static('dist'));
app.use(express.json());

// Simple proxy to backend API
app.post('/v1/chat/completions', async (req, res) => {
  try {
    // Forward to backend API
    const response = await fetch('http://localhost:8000/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(req.body)
    });
    const data = await response.json();
    res.json(data);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.listen(port, () => {
  console.log(`User UI server running at http://localhost:${port}`);
});

