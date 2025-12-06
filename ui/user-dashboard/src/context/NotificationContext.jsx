


import React, { createContext, useContext, useState, useCallback } from 'react'
import { v4 as uuidv4 } from 'uuid'

const NotificationContext = createContext()

export const useNotification = () => {
  return useContext(NotificationContext)
}

export const NotificationProvider = ({ children }) => {
  const [notifications, setNotifications] = useState([])

  const addNotification = useCallback((message, type = 'info', duration = 5000) => {
    const id = uuidv4()
    setNotifications(prev => [...prev, { id, message, type, duration }])

    // Auto-remove after duration
    setTimeout(() => {
      setNotifications(prev => prev.filter(notification => notification.id !== id))
    }, duration)
  }, [])

  const removeNotification = useCallback((id) => {
    setNotifications(prev => prev.filter(notification => notification.id !== id))
  }, [])

  return (
    <NotificationContext.Provider value={{ addNotification, removeNotification }}>
      {children}
      <div className="fixed top-4 right-4 z-50 space-y-4 w-full max-w-sm">
        {notifications.map(notification => (
          <div
            key={notification.id}
            className={`p-4 rounded-lg shadow-lg text-white ${
              notification.type === 'error' ? 'bg-red-600' :
              notification.type === 'success' ? 'bg-green-600' :
              notification.type === 'warning' ? 'bg-yellow-600' :
              'bg-blue-600'
            }`}
            role="alert"
            aria-live="polite"
          >
            <div className="flex justify-between items-start">
              <span>{notification.message}</span>
              <button
                onClick={() => removeNotification(notification.id)}
                className="ml-4 text-white opacity-70 hover:opacity-100"
                aria-label="Close notification"
              >
                ×
              </button>
            </div>
          </div>
        ))}
      </div>
    </NotificationContext.Provider>
  )
}


