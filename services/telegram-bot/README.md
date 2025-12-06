



# ZB AI Telegram Bot

## Overview

This Telegram bot provides comprehensive integration for ZB AI, including:

- User registration and authentication
- Admin alerts for important events
- User alerts for balance notifications
- Telegram Mini Apps integration
- TON payment support

## Features

### 1. User Registration

- Automatically registers new users when they start the bot
- Generates unique API keys for each user
- Notifies admin about new registrations

### 2. Admin Alerts

- Sends important system notifications to admin
- Configurable admin chat ID

### 3. User Alerts

- Balance notifications
- System updates
- Payment confirmations

### 4. Telegram Mini Apps

- Integrated web app interface
- Account management
- Statistics and analytics
- Settings and configuration

### 5. TON Payments

- TON coin payment integration
- Payment notifications
- Balance updates

## Setup

### 1. Configuration

Create a `.env` file with your configuration:

```
TELEGRAM_BOT_TOKEN=your-telegram-bot-token-here
ADMIN_CHAT_ID=your-admin-chat-id-here
PORT=3000
```

### 2. Installation

```bash
npm install
```

### 3. Running

```bash
npm start
```

### 4. Development

```bash
npm run dev
```

## Usage

### Commands

- `/start` - Register and get API key
- `/deposit_ton` - Deposit funds via TON

### Webhooks

- `/webhook/payment` - Payment notifications
- `/webhook/alert` - System alerts

## Integration

### 1. User Flow

1. User starts bot with `/start`
2. Bot registers user and sends API key
3. User can access Mini App for account management
4. User receives balance alerts when needed

### 2. Admin Flow

1. Admin receives notifications about new users
2. Admin receives system alerts
3. Admin can monitor user activity

### 3. Payment Flow

1. User initiates TON payment
2. Payment gateway processes transaction
3. Webhook notifies bot of payment status
4. User balance is updated
5. User receives confirmation

## Deployment

### Docker

```dockerfile
# Use official Node.js image
FROM node:20-alpine

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy source code
COPY . .

# Expose port
EXPOSE 3000

# Start the bot
CMD ["npm", "start"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  telegram-bot:
    build: ./services/telegram-bot
    ports:
      - "3000:3000"
    environment:
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - ADMIN_CHAT_ID=${ADMIN_CHAT_ID}
    restart: always
```

## Future Enhancements

- Advanced analytics
- Multi-language support
- Group management
- Enhanced security features
- Integration with more payment methods

## License

MIT





