# API Reference

## Authentication
- **Login**: `POST /auth/login`
- **Register**: `POST /auth/register`
- **2FA**: `POST /auth/2fa/enable`, `POST /auth/2fa/verify`

## API Keys
- **Create**: `POST /user/api-keys`
- **List**: `GET /user/api-keys`
- **Rotate**: `POST /user/api-keys/{id}/rotate`
- **Usage**: `GET /user/api-keys/{id}/usage`

## Billing
- **Balance**: `GET /billing/balance`
- **Top Up**: `POST /billing/create-checkout`
- **History**: `GET /billing/history`
- **Subscriptions**: `POST /billing/subscribe`

## Usage
- **Analytics**: `GET /billing/usage`
- **Models**: `GET /billing/models`
