


const express = require('express');
const app = express();
const port = 8000;

app.use(express.json());

app.post('/v1/chat/completions', (req, res) => {
  const { messages } = req.body;

  res.json({
    id: "chatcmpl-123",
    object: "chat.completion",
    created: Math.floor(Date.now() / 1000),
    model: "gpt-4o",
    choices: [
      {
        index: 0,
        message: {
          role: "assistant",
          content: "This is a mock response to: " + messages[0].content
        },
        finish_reason: "stop"
      }
    ],
    usage: {
      prompt_tokens: 10,
      completion_tokens: 15,
      total_tokens: 25
    }
  });
});

app.listen(port, () => {
  console.log(`Mock backend API running at http://localhost:${port}`);
});


