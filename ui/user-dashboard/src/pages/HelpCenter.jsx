













import { useState } from 'react'
import { Link } from 'react-router-dom'

const helpSections = [
  {
    title: 'Getting Started',
    content: `
      <h3>Welcome to our service!</h3>
      <p>This guide will help you get started with our platform.</p>
      <ol>
        <li>Register for an account</li>
        <li>Verify your email</li>
        <li>Log in to the dashboard</li>
        <li>Create your first API key</li>
      </ol>
    `
  },
  {
    title: 'API Key Management',
    content: `
      <h3>Creating API Keys</h3>
      <p>To create an API key:</p>
      <ol>
        <li>Go to the API Keys page</li>
        <li>Click "Create New API Key"</li>
        <li>Set permissions and name</li>
        <li>Save the key securely</li>
      </ol>
      <h3>Key Security</h3>
      <p>Never share your API keys publicly. Use key rotation regularly.</p>
    `
  },
  {
    title: 'Payment & Billing',
    content: `
      <h3>Topping Up Balance</h3>
      <p>You can add funds via:</p>
      <ul>
        <li>YuKassa</li>
        <li>Stripe</li>
        <li>PayPal</li>
      </ul>
      <h3>Subscriptions</h3>
      <p>We offer monthly plans with discounts.</p>
    `
  },
  {
    title: 'Security Features',
    content: `
      <h3>Two-Factor Authentication</h3>
      <p>Enable 2FA for enhanced security:</p>
      <ol>
        <li>Go to Security Settings</li>
        <li>Click "Enable 2FA"</li>
        <li>Scan QR code with authenticator app</li>
        <li>Enter verification code</li>
      </ol>
      <h3>Session Management</h3>
      <p>Monitor and terminate active sessions.</p>
    `
  },
  {
    title: 'Troubleshooting',
    content: `
      <h3>Common Issues</h3>
      <ul>
        <li>API key not working: Check permissions and rotation</li>
        <li>Payment failed: Verify payment method and try again</li>
        <li>2FA issues: Use backup codes or contact support</li>
      </ul>
      <h3>Contact Support</h3>
      <p>Email: support@yourdomain.com</p>
    `
  }
]

export default function HelpCenter() {
  const [selectedSection, setSelectedSection] = useState(helpSections[0])

  return (
    <div className="max-w-6xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8">Help Center</h1>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
        {/* Sidebar Navigation */}
        <div className="bg-white p-6 rounded-xl shadow-lg">
          <h2 className="text-2xl font-bold mb-4">Topics</h2>
          <ul className="space-y-2">
            {helpSections.map((section, index) => (
              <li key={index}>
                <button
                  onClick={() => setSelectedSection(section)}
                  className={`w-full text-left p-3 rounded-lg hover:bg-gray-100 ${selectedSection.title === section.title ? 'bg-indigo-100 text-indigo-700' : ''}`}
                >
                  {section.title}
                </button>
              </li>
            ))}
          </ul>

          <div className="mt-8">
            <h2 className="text-xl font-bold mb-2">More Resources</h2>
            <ul className="space-y-2">
              <li>
                <Link to="/api-keys" className="text-indigo-600 hover:underline">API Keys</Link>
              </li>
              <li>
                <Link to="/security" className="text-indigo-600 hover:underline">Security Settings</Link>
              </li>
              <li>
                <a href="https://github.com/your/repo/wiki" target="_blank" rel="noopener noreferrer" className="text-indigo-600 hover:underline">Full Documentation</a>
              </li>
            </ul>
          </div>
        </div>

        {/* Content Area */}
        <div className="md:col-span-3 bg-white p-8 rounded-xl shadow-lg">
          <h2 className="text-3xl font-bold mb-6">{selectedSection.title}</h2>
          <div className="prose max-w-none" dangerouslySetInnerHTML={{ __html: selectedSection.content }} />
        </div>
      </div>
    </div>
  )
}
















