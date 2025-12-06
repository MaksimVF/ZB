

# Routing Service Admin Integration

## Overview

This document describes the integration of the routing service administration interface into the main admin dashboard. The integration allows administrators to manage routing policies and head services through a unified interface.

## Integration Components

### 1. Routing Page Component

- **Location**: `/src/pages/Routing.jsx`
- **Features**:
  - Routing policy configuration
  - Head services management
  - New head registration
  - Real-time status updates

### 2. API Integration

- **Location**: `/src/api.js`
- **Endpoints**:
  - `GET /admin/routing/policy` - Get current routing policy
  - `PUT /admin/routing/policy` - Update routing policy
  - `GET /admin/routing/heads` - List all head services
  - `POST /admin/routing/heads` - Register new head service

### 3. Server Proxy

- **Location**: `server.js`
- **Function**: Proxies API requests to the routing service with proper authentication

### 4. Navigation

- **Location**: `/src/pages/Dashboard.jsx`
- **Feature**: Added quick link to Routing section in the main dashboard

## Technical Implementation

### Network Configuration

The admin dashboard is connected to both `client_network` and `server_network` to communicate with the routing service:

```yaml
ui:
  networks:
    - client_network
    - server_network
  depends_on:
    - routing-service
```

### API Proxy

The server acts as a proxy, forwarding requests to the routing service with authentication:

```javascript
app.get('/admin/routing/policy', async (req, res) => {
  // Forward request to routing-service
  const response = await axios.get('http://routing-service:8080/api/routing/policy', {
    headers: { 'X-Admin-Key': req.headers['x-admin-key'] }
  });
  res.json(response.data);
});
```

### UI Components

The Routing page reuses existing UI components and styles for consistency:

- Form controls with Tailwind CSS styling
- Responsive grid layout
- Consistent color scheme and typography

## Usage

1. Navigate to the admin dashboard
2. Click on the "Маршрутизация" link
3. Configure routing policies and manage head services

## Benefits

- Unified administration interface
- Consistent user experience
- Reuse of existing authentication and UI components
- Modular architecture allowing independent service development

## Future Enhancements

- Add real-time updates using WebSockets
- Implement advanced analytics for routing decisions
- Add visual representation of routing topology

