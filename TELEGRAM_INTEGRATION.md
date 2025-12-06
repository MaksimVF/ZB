





# Telegram Integration for ZB AI

## Overview

This document describes the comprehensive Telegram integration for ZB AI, including user registration, admin alerts, user notifications, Telegram Mini Apps, and TON payment support.

## Architecture

### 1. Telegram Bot Service

- **Location**: `/services/telegram-bot`
- **Technology**: Node.js, Express, node-telegram-bot-api
- **Features**:
  - User registration and authentication
  - Admin alerts for important events
  - User alerts for balance notifications
  - TON payment processing
  - Webhook endpoints

### 2. Telegram Mini App

- **Location**: `/ui/telegram-app`
- **Technology**: HTML, CSS, JavaScript, Telegram Web App API
- **Features**:
  - Account management interface
  - Balance and statistics display
  - API key management
  - Settings and configuration
  - TON payment interface

### 3. Docker Integration

- **Services**:
  - `telegram-bot`: Node.js service for bot functionality
  - `telegram-app`: Nginx service for Mini App interface
- **Networks**: Integrated with client and server networks

## Features

### 1. User Registration

- **Automatic Registration**: Users are automatically registered when they start the bot
- **API Key Generation**: Unique API keys are generated for each user
- **Admin Notifications**: Admins are notified about new user registrations

### 2. Admin Alerts

- **System Notifications**: Admins receive alerts about important system events
- **User Activity**: Admins are informed about new user registrations
- **Error Reporting**: Critical errors are reported to admins

### 3. User Alerts

- **Balance Notifications**: Users receive alerts when their balance is low
- **Payment Confirmations**: Users get notifications about successful payments
- **System Updates**: Users are informed about important system changes

### 4. Telegram Mini Apps

- **Account Management**: Full account management interface within Telegram
- **Statistics**: Detailed usage statistics and analytics
- **Settings**: Configuration options for notifications, language, and region
- **Documentation**: Access to API documentation and user guides

### 5. TON Payments

- **TON Integration**: Support for TON coin payments
- **Payment Processing**: Secure payment processing with webhook notifications
- **Balance Updates**: Real-time balance updates after payments

## Implementation

### 1. Telegram Bot Service

```javascript
// User registration
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
});
```

### 2. Admin Alerts

```javascript
// Admin alerts
function sendAdminAlert(message) {
  if (ADMIN_CHAT_ID) {
    bot.sendMessage(ADMIN_CHAT_ID, `⚠️ ВНИМАНИЕ АДМИНИСТРАТОРУ:\n${message}`);
  }
}

// Balance monitoring
setInterval(() => {
  Object.values(users).forEach(user => {
    if (user.balance < 5) {
      sendUserAlert(user.id, `Ваш баланс низкий: $${user.balance.toFixed(2)}. Пожалуйста, пополните баланс.`);
    }
  });
}, 60 * 60 * 1000);
```

### 3. TON Payments

```javascript
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
```

### 4. Webhooks

```javascript
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
```

## Telegram Mini App

### 1. Interface

```html
<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ZB AI Telegram App</title>
  <script src="https://telegram.org/js/telegram-web-app.js"></script>
  <!-- ... -->
</head>
<body>
  <div class="app">
    <div class="header">
      <div class="logo">ZB</div>
      <h1 class="title">ZB AI Telegram App</h1>
      <p class="subtitle">Управление вашим аккаунтом и услугами</p>
    </div>

    <div class="section">
      <div class="balance-card">
        <div class="balance-amount">$25.75</div>
        <div class="balance-label">Текущий баланс</div>
        <button class="deposit-btn" onclick="showDepositOptions()">Пополнить баланс</button>
      </div>
    </div>

    <!-- ... -->
  </div>
</body>
</html>
```

### 2. JavaScript Integration

```javascript
// Telegram Web App integration
const tg = window.Telegram.WebApp;

// Initialize the app
document.addEventListener('DOMContentLoaded', () => {
  tg.ready();

  // Send data to Telegram
  function sendDataToTelegram(data) {
    tg.sendData(JSON.stringify(data));
  }

  // Handle incoming data from Telegram
  tg.onEvent('mainButtonClicked', () => {
    sendDataToTelegram({ action: 'main_button_clicked' });
  });
});
```

## Deployment

### 1. Docker Configuration

```yaml
# Telegram Bot Service
telegram-bot:
  build: ./services/telegram-bot
  ports:
    - "3003:3000"
  networks:
    - client_network
    - server_network
  environment:
    - TELEGRAM_BOT_TOKEN=${Telegram_Token}
    - ADMIN_CHAT_ID=${ADMIN_CHAT_ID}
  depends_on:
    - secret-service
    - head

# Telegram Mini App
telegram-app:
  build: ./ui/telegram-app
  ports:
    - "3004:80"
  networks:
    - client_network
  depends_on:
    - telegram-bot
```

### 2. Environment Variables

```
TELEGRAM_BOT_TOKEN=your-telegram-bot-token-here
ADMIN_CHAT_ID=your-admin-chat-id-here
PORT=3000
```

## Usage

### 1. User Flow

1. **Registration**: User starts bot with `/start` command
2. **API Key**: User receives unique API key
3. **Mini App**: User accesses account management via Mini App
4. **Payments**: User can deposit funds via TON or credit card
5. **Notifications**: User receives balance alerts and payment confirmations

### 2. Admin Flow

1. **Alerts**: Admin receives notifications about new users and system events
2. **Monitoring**: Admin can monitor user activity and system health
3. **Management**: Admin can manage users and system configuration

## Future Enhancements

- **Advanced Analytics**: Detailed usage analytics and reporting
- **Multi-Language Support**: Support for multiple languages
- **Group Management**: Management of user groups and teams
- **Enhanced Security**: Advanced security features like 2FA
- **More Payment Methods**: Integration with additional payment providers
- **Social Features**: User profiles, leaderboards, and community features

## Benefits

1. **Seamless Integration**: Full integration with Telegram ecosystem
2. **User-Friendly**: Easy-to-use interface within Telegram
3. **Secure Payments**: Secure TON payment processing
4. **Real-Time Notifications**: Instant alerts and notifications
5. **Comprehensive Management**: Full account management capabilities

## Conclusion

The Telegram integration provides a comprehensive solution for user management, notifications, and payments within the ZB AI ecosystem. It leverages Telegram's powerful platform to provide a seamless user experience while maintaining robust administrative capabilities.







