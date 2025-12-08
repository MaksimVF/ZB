




# ZB AI Telegram Mini App

## Overview

This is a Telegram Mini App interface for ZB AI that provides users with a comprehensive account management experience directly within Telegram.

## Features

### 1. Account Management

- View and manage API keys
- Check account balance
- Deposit funds via TON and credit card

### 2. Statistics

- View request statistics
- Track spending
- Monitor account activity

### 3. Settings

- Manage notifications
- Change language
- Configure region settings

### 4. Documentation

- Access API documentation
- View user guides
- Get help and support

### 5. TON Payments

- Integrated TON payment system
- Secure transactions
- Real-time balance updates

## Usage

### 1. Accessing the App

- Open Telegram
- Start the ZB AI bot
- Click "Open App" button

### 2. Balance Management

- View current balance
- Deposit funds via TON or credit card
- Track spending history

### 3. API Key Management

- View your API key
- Copy to clipboard
- Regenerate if needed

### 4. Statistics

- View request counts
- Track spending
- Monitor account activity

### 5. Settings

- Configure notifications
- Change language
- Set region preferences

## Integration

### 1. Telegram Web App

The app uses Telegram's Web App API for seamless integration:

```html
<script src="https://telegram.org/js/telegram-web-app.js"></script>
```

### 2. Data Exchange

- Send data to Telegram bot
- Receive data from bot
- Handle events and callbacks

### 3. Payment Integration

- TON payment gateway
- Credit card processing
- Secure transactions

## Deployment

### 1. Hosting

Host the app on your domain:

```
https://your-domain.com/telegram-app
```

### 2. Bot Configuration

Configure your Telegram bot to use the app URL:

```javascript
const inlineKeyboard = {
  inline_keyboard: [
    [
      {
        text: '📱 Открыть приложение',
        web_app: { url: 'https://your-domain.com/telegram-app' }
      }
    ]
  ]
};
```

### 3. Security

- Use HTTPS for all connections
- Implement proper authentication
- Secure API endpoints

## Future Enhancements

- Advanced analytics
- Multi-language support
- Enhanced payment options
- Social features
- Integration with more services

## License

MIT






