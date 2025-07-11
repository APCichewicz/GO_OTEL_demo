import { useState, useEffect } from 'react'
import './App.css'

type SessionData = {
  token: string;
  user_id: number;
  email: string;
  name: string;
  provider: string;
  issued_at: string;
  expires_at: string;
}

function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [sessionData, setSessionData] = useState<SessionData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const API_BASE_URL = 'http://localhost:8080'

  useEffect(() => {
    const pathname = window.location.pathname
    if (pathname === '/auth/success') {
      checkAuthStatus()
    }
  }, [])

  const checkAuthStatus = async () => {
    try {
      const response = await fetch(`${API_BASE_URL}/auth/user`, {
        credentials: 'include'
      })
      
      if (response.ok) {
        const data = await response.json()
        setSessionData(data)
        setIsLoggedIn(true)
      }
    } catch (err) {
      console.error('Error checking auth status:', err)
    }
  }

  const handleLogin = async () => {
    setLoading(true)
    setError(null)
    
    try {
      window.location.href = `${API_BASE_URL}/auth/login/authentik`
    } catch (err) {
      setError('Failed to initiate login')
      setLoading(false)
    }
  }

  if (isLoggedIn && sessionData) {
    return (
      <div className="success-container">
        <h1>✅ Successful Login</h1>
        <div className="token-info">
          <h3>Session Information:</h3>
          <div className="session-details">
            <p><strong>Token:</strong> {sessionData.token}</p>
            <p><strong>User ID:</strong> {sessionData.user_id}</p>
            <p><strong>Email:</strong> {sessionData.email}</p>
            <p><strong>Name:</strong> {sessionData.name}</p>
            <p><strong>Provider:</strong> {sessionData.provider}</p>
            <p><strong>Issued At:</strong> {new Date(sessionData.issued_at).toLocaleString()}</p>
            <p><strong>Expires At:</strong> {new Date(sessionData.expires_at).toLocaleString()}</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="login-container">
      <h1>Authentication Demo</h1>
      <div className="card">
        <button 
          onClick={handleLogin}
          disabled={loading}
          className="login-button"
        >
          {loading ? 'Initiating Login...' : 'Login'}
        </button>
        {error && <p className="error">{error}</p>}
      </div>
    </div>
  )
}

export default App
