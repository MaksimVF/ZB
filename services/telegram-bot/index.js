




require('dotenv').config();
const TelegramBot = require('node-telegram-bot-api');
const express = require('express');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');

// Initialize Telegram bot
const token = process.env.TELEGRAM_BOT_TOKEN;
const bot = new TelegramBot(token, { polling: true });

// Express app for webhook
const app = express();
const PORT = process.env.PORT || 3000;

// User database (in-memory for now)
const users = {};

// Admin chat ID (configure in .env)
const ADMIN_CHAT_ID = process.env.ADMIN_CHAT_ID || '';

// Start express server
app.listen(PORT, () => {
  console.log(`Telegram bot server running on port ${PORT}`);
});

// Telegram bot commands
bot.onText(/\/start/, (msg) => {
  const chatId = msg.chat.id;
  const userId = msg.from.id;
  const username = msg.from.username || msg.from.first_name;

  // Register user if not exists
  if (!users[userId]) {
    users[userId] = {
      id: userId,
      username,
      chatId,
      balance: 0,
      registeredAt: new Date(),
      lastActive: new Date(),
      telegramId: userId,
      apiKey: uuidv4(),
    };

    // Notify admin about new user
    if (ADMIN_CHAT_ID) {
      bot.sendMessage(ADMIN_CHAT_ID, `🚀 Новый пользователь зарегистрировался:\nID: ${userId}\nИмя: ${username}\nЧат ID: ${chatId}`);
    }
  }

  // Send welcome message with Mini App button
  const welcomeMessage = `
🎉 Добро пожаловать в ZB AI, ${username}!

🔹 Ваш API ключ: \`${users[userId].apiKey}\`
🔹 Текущий баланс: $${users[userId].balance.toFixed(2)}

📱 Используйте наше приложение для управления аккаунтом:
`;

  const inlineKeyboard = {
    inline_keyboard: [
      [
        {
          text: '📱 Открыть приложение',
          web_app: { url: 'https://your-domain.com/telegram-app' }
        }
      ],
      [
        {
          text: '💰 Пополнить баланс',
          callback_data: 'deposit'
        },
        {
          text: '📊 Статистика',
          callback_data: 'stats'
        }
      ],
      [
        {
          text: '🔧 Настройки',
          callback_data: 'settings'
        },
        {
          text: '📖 Документация',
          callback_data: 'docs'
        }
      ]
    ]
  };

  bot.sendMessage(chatId, welcomeMessage, {
    parse_mode: 'Markdown',
    reply_markup: JSON.stringify(inlineKeyboard)
  });
});

// Handle callback queries
bot.on('callback_query', (query) => {
  const chatId = query.message.chat.id;
  const data = query.data;

  switch (data) {
    case 'deposit':
      sendDepositOptions(chatId);
      break;
    case 'stats':
      sendStats(chatId);
      break;
    case 'settings':
      sendSettings(chatId);
      break;
    case 'docs':
      sendDocs(chatId);
      break;
    default:
      break;
  }
});

// Send deposit options
function sendDepositOptions(chatId) {
  const message = '💰 Выберите способ пополнения:';
  const inlineKeyboard = {
    inline_keyboard: [
      [
        {
          text: '💎 TON Coin',
          callback_data: 'deposit_ton'
        },
        {
          text: '💳 Кредитная карта',
          callback_data: 'deposit_card'
        }
      ],
      [
        {
          text: '🔙 Назад',
          callback_data: 'back'
        }
      ]
    ]
  };

  bot.sendMessage(chatId, message, {
    reply_markup: JSON.stringify(inlineKeyboard)
  });
}

// Send user stats
function sendStats(chatId) {
  const userId = Object.keys(users).find(key => users[key].chatId === chatId);
  if (!userId) return;

  const user = users[userId];
  const message = `
📊 Ваша статистика:

🔹 API ключ: \`${user.apiKey}\`
🔹 Баланс: $${user.balance.toFixed(2)}
🔹 Зарегистрирован: ${user.registeredAt.toLocaleString()}
🔹 Последняя активность: ${user.lastActive.toLocaleString()}
`;

  const inlineKeyboard = {
    inline_keyboard: [
      [
        {
          text: '🔙 Назад',
          callback_data: 'back'
        }
      ]
    ]
  };

  bot.sendMessage(chatId, message, {
    parse_mode: 'Markdown',
    reply_markup: JSON.stringify(inlineKeyboard)
  });
}

// Send settings
function sendSettings(chatId) {
  const message = '⚙️ Настройки аккаунта:';
  const inlineKeyboard = {
    inline_keyboard: [
      [
        {
          text: '🔄 Сменить API ключ',
          callback_data: 'reset_api_key'
        }
      ],
      [
        {
          text: '🔙 Назад',
          callback_data: 'back'
        }
      ]
    ]
  };

  bot.sendMessage(chatId, message, {
    reply_markup: JSON.stringify(inlineKeyboard)
  });
}

// Send documentation
function sendDocs(chatId) {
  const message = '📖 Документация и полезные ссылки:';
  const inlineKeyboard = {
    inline_keyboard: [
      [
        {
          text: '🌐 API Документация',
          url: 'https://your-domain.com/api-docs'
        }
      ],
      [
        {
          text: '📄 Руководство пользователя',
          url: 'https://your-domain.com/user-guide'
        }
      ],
      [
        {
          text: '🔙 Назад',
          callback_data: 'back'
        }
      ]
    ]
  };

  bot.sendMessage(chatId, message, {
    reply_markup: JSON.stringify(inlineKeyboard)
  });
}

// Handle TON payments
bot.onText(/\/deposit_ton/, (msg) => {
  const chatId = msg.chat.id;
  const userId = msg.from.id;

  // Generate payment invoice
  const paymentId = uuidv4();
  const amount = 10; // Default amount in USD

  // In a real implementation, this would create a TON payment request
  const paymentLink = `https://ton-payment-gateway.com/pay?amount=${amount}&currency=USD&paymentId=${paymentId}`;

  const message = `
💰 Пополнение баланса через TON

🔹 Сумма: $${amount.toFixed(2)}
🔹 ID платежа: ${paymentId}

🔗 Оплата: ${paymentLink}

После оплаты, ваш баланс будет обновлен автоматически.
`;

  bot.sendMessage(chatId, message);
});

// Admin alerts
function sendAdminAlert(message) {
  if (ADMIN_CHAT_ID) {
    bot.sendMessage(ADMIN_CHAT_ID, `⚠️ ВНИМАНИЕ АДМИНИСТРАТОРУ:\n${message}`);
  }
}

// User alerts
function sendUserAlert(userId, message) {
  const user = users[userId];
  if (user) {
    bot.sendMessage(user.chatId, `⚠️ ВНИМАНИЕ:\n${message}`);
  }
}

// Balance monitoring (example)
setInterval(() => {
  Object.values(users).forEach(user => {
    if (user.balance < 5) {
      sendUserAlert(user.id, `Ваш баланс низкий: $${user.balance.toFixed(2)}. Пожалуйста, пополните баланс.`);
    }
  });
}, 60 * 60 * 1000); // Check every hour

// Webhook endpoint for payment notifications
app.post('/webhook/payment', express.json(), (req, res) => {
  const { paymentId, userId, amount, status } = req.body;

  if (status === 'completed') {
    if (users[userId]) {
      users[userId].balance += amount;
      sendUserAlert(userId, `✅ Ваш баланс пополнен на $${amount.toFixed(2)}. Текущий баланс: $${users[userId].balance.toFixed(2)}`);
    }
  }

  res.status(200).send('OK');
});

// Webhook endpoint for system alerts
app.post('/webhook/alert', express.json(), (req, res) => {
  const { type, message } = req.body;

  if (type === 'admin') {
    sendAdminAlert(message);
  } else if (type === 'user' && req.body.userId) {
    sendUserAlert(req.body.userId, message);
  }

  res.status(200).send('OK');
});

console.log('Telegram bot is running...');




