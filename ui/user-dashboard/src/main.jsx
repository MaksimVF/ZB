





import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import ApiKeyManager from './components/ApiKeyManager';

const App = () => {
  // In a real app, you'd get this from authentication
  const userId = 'user-123';

  return (
    <Router>
      <div className="app">
        <header>
          <h1>User Dashboard</h1>
        </header>

        <main>
          <Routes>
            <Route path="/" element={<ApiKeyManager userId={userId} />} />
            <Route path="/api-keys" element={<ApiKeyManager userId={userId} />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
};

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(<App />);





