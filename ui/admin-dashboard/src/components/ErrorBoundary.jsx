






import React from 'react'

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
      showDetails: false
    }
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error: error }
  }

  componentDidCatch(error, errorInfo) {
    console.error("Error caught by ErrorBoundary:", error, errorInfo)
    this.setState({ errorInfo })
  }

  toggleDetails = () => {
    this.setState(prevState => ({ showDetails: !prevState.showDetails }))
  }

  render() {
    if (this.state.hasError) {
      return (
        <div
          className="flex flex-col items-center justify-center h-screen bg-red-50"
          role="alert"
          aria-live="assertive"
        >
          <h1 className="text-3xl font-bold text-red-600 mb-4">Something went wrong</h1>
          <p className="text-gray-600 mb-6">Please refresh the page or contact support if the problem persists.</p>

          <div className="mb-6">
            <button
              onClick={() => window.location.reload()}
              className="px-6 py-3 bg-red-600 text-white rounded-lg hover:bg-red-700 mr-4"
              aria-label="Refresh the page"
            >
              Refresh Page
            </button>

            <button
              onClick={this.toggleDetails}
              className="px-6 py-3 bg-gray-300 text-gray-700 rounded-lg hover:bg-gray-400"
              aria-label={this.state.showDetails ? "Hide error details" : "Show error details"}
            >
              {this.state.showDetails ? 'Hide Details' : 'Show Details'}
            </button>
          </div>

          {this.state.showDetails && (
            <div className="bg-white p-6 rounded-lg shadow-md w-full max-w-2xl mt-4 text-left">
              <h2 className="text-xl font-semibold mb-4 text-red-600">Error Details</h2>

              {this.state.error && (
                <div className="mb-4">
                  <h3 className="font-semibold">Error Message:</h3>
                  <p className="text-sm text-gray-700">{this.state.error.message}</p>
                </div>
              )}

              {this.state.errorInfo && (
                <div>
                  <h3 className="font-semibold">Component Stack:</h3>
                  <pre className="text-sm text-gray-700 overflow-auto max-h-60">{this.state.errorInfo.componentStack}</pre>
                </div>
              )}
            </div>
          )}
        </div>
      )
    }

    return this.props.children
  }
}

export default ErrorBoundary




