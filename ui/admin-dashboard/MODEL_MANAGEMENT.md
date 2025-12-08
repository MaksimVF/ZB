


# Model Management Feature

## Overview

This document describes the Model Management feature that allows administrators to manage LLM models, assign regions, and configure model-specific settings through a dedicated interface.

## Features

### 1. Model Management Page

- **Location**: `/src/pages/Models.jsx`
- **Features**:
  - View, create, edit, and delete models
  - Assign regions to models
  - Configure model endpoints and access settings
  - Set priority head services for each model
  - Manage API keys and metadata

### 2. Region Assignment

- **Manual Region Selection**: Assign regions to models using a dropdown selector
- **Region Codes**: Uses standard region codes (us, eu, ru, cn, br, etc.)
- **Examples**:
  - Mistral 7B → EU region
  - Kimi 13B → CN region
  - Gemini Pro → US region

### 3. Priority Head Selection

- **Head Service Assignment**: Select priority head service for each model
- **Region-Aware**: Shows head services with their regions for easy selection
- **Load Balancing**: Allows configuration of primary access points for models

### 4. Model Configuration

- **Endpoint Management**: Configure API endpoints for each model
- **API Key Management**: Securely store and manage API keys
- **Local Inference**: Toggle for local inference capability
- **Metadata**: Store additional model information in JSON format

### 5. API Integration

- **Endpoints**:
  - `GET /admin/models` - List all models
  - `POST /admin/models` - Create new model
  - `PUT /admin/models/:model_id` - Update model
  - `DELETE /admin/models/:model_id` - Delete model

### 6. Navigation

- **Quick Access**: Added link to dashboard for easy navigation
- **Route**: `/admin/models` for direct access

## Implementation

### Model Management Page

The Models.jsx page provides a comprehensive interface:

```jsx
// Model list with region and head service information
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
  {models.map(model => (
    <div key={model.model_id} className="bg-gray-50 dark:bg-gray-700 p-6 rounded-xl shadow-md">
      <h3 className="text-xl font-semibold mb-3 dark:text-white">{model.name}</h3>
      <p><strong>Region:</strong> {model.region}</p>
      <p><strong>Priority Head:</strong> {model.priority_head}</p>
      <p><strong>Endpoint:</strong> {model.endpoint}</p>
    </div>
  ))}
</div>
```

### Region and Head Selection

```jsx
// Region selection dropdown
<select name="region" value={model.region} onChange={handleChange}>
  {REGIONS.map(region => (
    <option key={region.code} value={region.code}>
      {region.name} ({region.code})
    </option>
  ))}
</select>

// Head service selection
<select name="priority_head" value={model.priority_head} onChange={handleChange}>
  {heads.map(head => (
    <option key={head.head_id} value={head.head_id}>
      {head.head_id} ({head.region})
    </option>
  ))}
</select>
```

## Benefits

1. **Centralized Management**: All model configurations in one place
2. **Region-Based Routing**: Easy assignment of models to specific regions
3. **Priority Configuration**: Set primary access points for each model
4. **Future-Ready**: Includes local inference configuration for future use
5. **Consistent UI**: Follows existing admin dashboard design patterns

## Usage Examples

1. **Assign Mistral to EU**:
   - Model: Mistral 7B
   - Region: eu (Europe)
   - Priority Head: head-service-eu

2. **Assign Kimi to China**:
   - Model: Kimi 13B
   - Region: cn (China)
   - Priority Head: head-service-cn

3. **Assign Gemini to US**:
   - Model: Gemini Pro
   - Region: us (United States)
   - Priority Head: head-service-us

## Future Enhancements

- Integration with actual model services
- Real-time model status monitoring
- Advanced routing analytics
- Model performance metrics
- Automated region detection based on usage patterns


