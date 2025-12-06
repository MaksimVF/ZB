

const axios = require('axios');

async function testRoutingIntegration() {
  try {
    console.log('Testing routing service integration...');

    // Test getting routing policy
    const policyResponse = await axios.get('http://localhost:8083/api/routing/policy', {
      headers: { 'X-Admin-Key': 'superadmin2025' }
    });
    console.log('Routing policy:', policyResponse.data);

    // Test getting head services
    const headsResponse = await axios.get('http://localhost:8083/api/routing/heads', {
      headers: { 'X-Admin-Key': 'superadmin2025' }
    });
    console.log('Head services:', headsResponse.data);

    // Test updating routing policy
    const updateResponse = await axios.put('http://localhost:8083/api/routing/policy', {
      default_strategy: 'round_robin',
      enable_geo_routing: true,
      enable_load_balancing: true,
      enable_model_specific: false,
      strategy_config: {}
    }, {
      headers: { 'X-Admin-Key': 'superadmin2025' }
    });
    console.log('Update response:', updateResponse.data);

    console.log('All tests passed!');
  } catch (error) {
    console.error('Error:', error.response?.data || error.message);
  }
}

testRoutingIntegration();

