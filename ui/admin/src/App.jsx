



import React, { useState, useEffect } from "react";

export default function App(){
  const [metrics, setMetrics] = useState(null);
  const [billing, setBilling] = useState(null);
  const [queues, setQueues] = useState(null);

  useEffect(() => {
    async function fetchData() {
      try {
        const [metricsRes, billingRes, queuesRes] = await Promise.all([
          fetch('/api/metrics'),
          fetch('/api/billing'),
          fetch('/api/queues')
        ]);

        setMetrics(await metricsRes.json());
        setBilling(await billingRes.json());
        setQueues(await queuesRes.json());
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    }

    fetchData();
  }, []);

  return <div style={{padding:20}}>
    <h1>Admin UI</h1>

    <div style={{display: 'flex', gap: '20px', marginTop: '20px'}}>
      <div style={{border: '1px solid #ccc', padding: '15px', borderRadius: '5px'}}>
        <h3>Metrics</h3>
        {metrics ? (
          <ul>
            <li>Total Requests: {metrics.totalRequests}</li>
            <li>Active Users: {metrics.activeUsers}</li>
            <li>Response Time: {metrics.responseTime}</li>
          </ul>
        ) : <p>Loading...</p>}
      </div>

      <div style={{border: '1px solid #ccc', padding: '15px', borderRadius: '5px'}}>
        <h3>Billing</h3>
        {billing ? (
          <ul>
            <li>Total Revenue: {billing.totalRevenue}</li>
            <li>Pending Payments: {billing.pendingPayments}</li>
            <li>Next Billing Cycle: {billing.nextBillingCycle}</li>
          </ul>
        ) : <p>Loading...</p>}
      </div>

      <div style={{border: '1px solid #ccc', padding: '15px', borderRadius: '5px'}}>
        <h3>Queues</h3>
        {queues ? (
          <ul>
            <li>Active Queues: {queues.activeQueues}</li>
            <li>Pending Jobs: {queues.pendingJobs}</li>
            <li>Completed Jobs: {queues.completedJobs}</li>
          </ul>
        ) : <p>Loading...</p>}
      </div>
    </div>
  </div>
}



