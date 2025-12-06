




require('dotenv').config();
const TelegramBot = require('node-telegram-bot-api');
const express = require('express');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');
const { createClient } = require('redis');
const rateLimit = require('express-rate-limit');

// Initialize Redis client
const redisClient = createClient({
  url: process.env.REDIS_URL || 'redis://localhost:6379'
});

redisClient.on('error', (err) => console.error('Redis Client Error', err));

(async () => {
  await redisClient.connect();
  console.log('Connected to Redis');
})();

// Initialize Telegram bot
const token = process.env.TELEGRAM_BOT_TOKEN;
if (!token) {
  throw new Error('TELEGRAM_BOT_TOKEN is not set in environment variables');
}
const bot = new TelegramBot(token, { polling: true });

// Express app for webhook
const app = express();
const PORT = process.env.PORT || 3000;

// Admin chat ID (configure in .env)
const ADMIN_CHAT_ID = process.env.ADMIN_CHAT_ID || '';

// Rate limiting for API endpoints
const apiLimiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100, // limit each IP to 100 requests per windowMs
  message: 'Too many requests from this IP, please try again later'
});

// Apply rate limiting to webhook endpoints
app.use('/webhook/', apiLimiter);

// Start express server
app.listen(PORT, () => {
  console.log(`Telegram bot server running on port ${PORT}`);
});

// Telegram bot commands
bot.onText(/\/start/, async (msg) => {
  try {
    const chatId = msg.chat.id;
    const userId = msg.from.id;
    const username = msg.from.username || msg.from.first_name;

    // Register user if not exists
    const userKey = `user:${userId}`;
    let user = await redisClient.get(userKey);

    if (!user) {
      user = {
        id: userId,
        username,
        chatId,
        balance: 0,
        registeredAt: new Date().toISOString(),
        lastActive: new Date().toISOString(),
        telegramId: userId,
        apiKey: uuidv4(),
      };

      // Store user in Redis
      await redisClient.set(userKey, JSON.stringify(user));
      await redisClient.sAdd('users', userId);

      // Notify admin about new user
      if (ADMIN_CHAT_ID) {
        bot.sendMessage(ADMIN_CHAT_ID, `🚀 Новый пользователь зарегистрировался:\nID: ${userId}\nИмя: ${username}\nЧат ID: ${chatId}`);
      }
    } else {
      user = JSON.parse(user);
      // Update last active time
      user.lastActive = new Date().toISOString();
      await redisClient.set(userKey, JSON.stringify(user));
    }

    // Send welcome message with Mini App button
    const welcomeMessage = `
🎉 Добро пожаловать в ZB AI, ${username}!

🔹 Ваш API ключ: \`${user.apiKey}\`
🔹 Текущий баланс: $${user.balance.toFixed(2)}

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
  } catch (error) {
    console.error('Error in /start command:', error);
    bot.sendMessage(msg.chat.id, 'Произошла ошибка при обработке вашего запроса. Пожалуйста, попробуйте позже.');
  }
});

// Handle callback queries
bot.on('callback_query', async (query) => {
  try {
    const chatId = query.message.chat.id;
    const data = query.data;

    switch (data) {
      case 'deposit':
        await sendDepositOptions(chatId);
        break;
      case 'stats':
        await sendStats(chatId);
        break;
      case 'settings':
        await sendSettings(chatId);
        break;
      case 'docs':
        await sendDocs(chatId);
        break;
      case 'deposit_ton':
        await handleTonDeposit(chatId);
        break;
      case 'deposit_card':
        await handleCardDeposit(chatId);
        break;
      case 'reset_api_key':
        await handleResetApiKey(chatId);
        break;
      case 'back':
        await sendMainMenu(chatId);
        break;
      default:
        break;
    }
  } catch (error) {
    console.error('Error in callback query handler:', error);
    bot.sendMessage(query.message.chat.id, 'Произошла ошибка при обработке вашего запроса. Пожалуйста, попробуйте позже.');
  }
});

// Send deposit options
async function sendDepositOptions(chatId) {
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
async function sendStats(chatId) {
  try {
    const userId = await getUserIdByChatId(chatId);
    if (!userId) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    const user = await getUserById(userId);
    if (!user) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    const message = `
📊 Ваша статистика:

🔹 API ключ: \`${user.apiKey}\`
🔹 Баланс: $${user.balance.toFixed(2)}
🔹 Зарегистрирован: ${new Date(user.registeredAt).toLocaleString()}
🔹 Последняя активность: ${new Date(user.lastActive).toLocaleString()}
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
  } catch (error) {
    console.error('Error in sendStats:', error);
    bot.sendMessage(chatId, 'Произошла ошибка при получении вашей статистики. Пожалуйста, попробуйте позже.');
  }
}

// Send settings
async function sendSettings(chatId) {
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
async function sendDocs(chatId) {
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

// Handle TON deposit
async function handleTonDeposit(chatId) {
  try {
    const userId = await getUserIdByChatId(chatId);
    if (!userId) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    // Generate payment invoice
    const paymentId = uuidv4();
    const amount = 10; // Default amount in USD

    // In a real implementation, this would create a TON payment request
    const paymentLink = `https://ton-payment-gateway.com/pay?amount=${amount}&currency=USD&paymentId=${paymentId}&userId=${userId}`;

    const message = `
💰 Пополнение баланса через TON

🔹 Сумма: $${amount.toFixed(2)}
🔹 ID платежа: ${paymentId}

🔗 Оплата: ${paymentLink}

После оплаты, ваш баланс будет обновлен автоматически.
`;

    bot.sendMessage(chatId, message);
  } catch (error) {
    console.error('Error in handleTonDeposit:', error);
    bot.sendMessage(chatId, 'Произошла ошибка при обработке вашего платежа. Пожалуйста, попробуйте позже.');
  }
}

// Handle card deposit
async function handleCardDeposit(chatId) {
  try {
    const userId = await getUserIdByChatId(chatId);
    if (!userId) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    // In a real implementation, this would redirect to a secure payment gateway
    const paymentLink = `https://your-domain.com/payment/card?userId=${userId}`;

    const message = `
💳 Пополнение баланса через кредитную карту

🔗 Оплата: ${paymentLink}

После оплаты, ваш баланс будет обновлен автоматически.
`;

    bot.sendMessage(chatId, message);
  } catch (error) {
    console.error('Error in handleCardDeposit:', error);
    bot.sendMessage(chatId, 'Произошла ошибка при обработке вашего платежа. Пожалуйста, попробуйте позже.');
  }
}

// Handle API key reset
async function handleResetApiKey(chatId) {
  try {
    const userId = await getUserIdByChatId(chatId);
    if (!userId) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    const user = await getUserById(userId);
    if (!user) {
      bot.sendMessage(chatId, 'Пользователь не найден. Пожалуйста, начните с команды /start');
      return;
    }

    // Generate new API key
    user.apiKey = uuidv4();
    await updateUser(userId, user);

    bot.sendMessage(chatId, `✅ Ваш API ключ был успешно сменен. Новый ключ: \`${user.apiKey}\``, {
      parse_mode: 'Markdown'
    });
  } catch (error) {
    console.error('Error in handleResetApiKey:', error);
    bot.sendMessage(chatId, 'Произошла ошибка при смене вашего API ключа. Пожалуйста, попробуйте позже.');
  }
}

// Send main menu
async function sendMainMenu(chatId) {
  const message = '🏠 Главное меню:';
  const inlineKeyboard = {
    inline_keyboard: [
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

  bot.sendMessage(chatId, message, {
    reply_markup: JSON.stringify(inlineKeyboard)
  });
}

// Helper function to get user by chat ID
async function getUserIdByChatId(chatId) {
  const userKeys = await redisClient.keys('user:*');
  for (const key of userKeys) {
    const user = JSON.parse(await redisClient.get(key));
    if (user.chatId === chatId) {
      return user.id;
    }
  }
  return null;
}

// Helper function to get user by ID
async function getUserById(userId) {
  const userKey = `user:${userId}`;
  const user = await redisClient.get(userKey);
  return user ? JSON.parse(user) : null;
}

// Helper function to update user
async function updateUser(userId, userData) {
  const userKey = `user:${userId}`;
  await redisClient.set(userKey, JSON.stringify(userData));
}

// Handle TON payments
bot.onText(/\/deposit_ton/, async (msg) => {
  try {
    const chatId = msg.chat.id;
    const userId = msg.from.id;

    // Generate payment invoice
    const paymentId = uuidv4();
    const amount = 10; // Default amount in USD

    // Store payment request in Redis for validation
    const paymentKey = `payment:${paymentId}`;
    await redisClient.set(paymentKey, JSON.stringify({
      userId,
      amount,
      status: 'pending',
      createdAt: new Date().toISOString()
    }), {
      EX: 3600 // Expire after 1 hour
    });

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
  } catch (error) {
    console.error('Error in /deposit_ton command:', error);
    bot.sendMessage(msg.chat.id, 'Произошла ошибка при обработке вашего платежа. Пожалуйста, попробуйте позже.');
  }
});

// Admin alerts
async function sendAdminAlert(message) {
  if (ADMIN_CHAT_ID) {
    try {
      await bot.sendMessage(ADMIN_CHAT_ID, `⚠️ ВНИМАНИЕ АДМИНИСТРАТОРУ:\n${message}`);
    } catch (error) {
      console.error('Error sending admin alert:', error);
    }
  }
}

// User alerts
async function sendUserAlert(userId, message) {
  try {
    const user = await getUserById(userId);
    if (user) {
      await bot.sendMessage(user.chatId, `⚠️ ВНИМАНИЕ:\n${message}`);
    }
  } catch (error) {
    console.error('Error sending user alert:', error);
  }
}

// Balance monitoring (example)
setInterval(async () => {
  try {
    const userKeys = await redisClient.keys('user:*');
    for (const key of userKeys) {
      const user = JSON.parse(await redisClient.get(key));
      if (user.balance < 5) {
        await sendUserAlert(user.id, `Ваш баланс низкий: $${user.balance.toFixed(2)}. Пожалуйста, пополните баланс.`);
      }
    }
  } catch (error) {
    console.error('Error in balance monitoring:', error);
  }
}, 60 * 60 * 1000); // Check every hour

// Webhook endpoint for payment notifications
app.post('/webhook/payment', express.json(), async (req, res) => {
  try {
    const { paymentId, userId, amount, status, signature } = req.body;

    // Validate required fields
    if (!paymentId || !userId || !amount || !status) {
      return res.status(400).json({ error: 'Missing required fields' });
    }

    // Validate payment (in a real implementation, this would verify the signature)
    const paymentKey = `payment:${paymentId}`;
    const paymentData = await redisClient.get(paymentKey);

    if (!paymentData) {
      return res.status(404).json({ error: 'Payment not found' });
    }

    const payment = JSON.parse(paymentData);

    // Validate payment status and amount
    if (payment.status !== 'pending' || payment.userId !== userId || payment.amount !== amount) {
      return res.status(400).json({ error: 'Invalid payment data' });
    }

    if (status === 'completed') {
      const user = await getUserById(userId);
      if (user) {
        // Update user balance
        user.balance += amount;
        await updateUser(userId, user);

        // Update payment status
        payment.status = 'completed';
        await redisClient.set(paymentKey, JSON.stringify(payment), {
          EX: 86400 // Keep for 24 hours
        });

        // Notify user
        await sendUserAlert(userId, `✅ Ваш баланс пополнен на $${amount.toFixed(2)}. Текущий баланс: $${user.balance.toFixed(2)}`);
      }
    }

    res.status(200).json({ status: 'OK' });
  } catch (error) {
    console.error('Error in payment webhook:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Webhook endpoint for system alerts
app.post('/webhook/alert', express.json(), async (req, res) => {
  try {
    const { type, message, userId } = req.body;

    // Validate required fields
    if (!type || !message) {
      return res.status(400).json({ error: 'Missing required fields' });
    }

    if (type === 'admin') {
      await sendAdminAlert(message);
    } else if (type === 'user' && userId) {
      await sendUserAlert(userId, message);
    }

    res.status(200).json({ status: 'OK' });
  } catch (error) {
    console.error('Error in alert webhook:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

console.log('Telegram bot is running...');




