import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, NavLink, useNavigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { DomainProvider, useDomain } from './context/DomainContext';
import Login from './pages/Login';
import DNSProviders from './pages/DNSProviders';
import Domains from './pages/Domains';
import Services from './pages/Services';
import Emails from './pages/Emails';
import DNS from './pages/DNS';
import SSHKeys from './pages/SSHKeys';
import './App.css';

function PrivateRoute({ children }) {
  const { isAuthenticated, loading } = useAuth();

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return isAuthenticated ? children : <Navigate to="/login" />;
}

function Sidebar() {
  const navigate = useNavigate();
  const { currentDomain, clearDomain } = useDomain();

  const handleBackToDomains = () => {
    clearDomain();
    navigate('/domains');
  };

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <img src="/logo-hibana.png" alt="Hibana" className="sidebar-logo" />
      </div>

      <nav className="sidebar-nav">
        {currentDomain ? (
          <>
            <div className="sidebar-domain-header">
              <button className="sidebar-back-btn" onClick={handleBackToDomains} title="Back to domains">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M19 12H5M12 19l-7-7 7-7"/>
                </svg>
              </button>
              <span className="sidebar-domain-name">{currentDomain.name}</span>
            </div>
            <NavLink to="/services" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">📦</span>
              Services
            </NavLink>
            <NavLink to="/emails" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">📧</span>
              Email
            </NavLink>
            <NavLink to="/ssh-keys" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">🔑</span>
              SSH Keys
            </NavLink>
            <NavLink to="/dns" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">🌐</span>
              DNS Records
            </NavLink>
          </>
        ) : (
          <>
            <NavLink to="/dns-providers" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">🔌</span>
              DNS Providers
            </NavLink>
            <NavLink to="/domains" className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <span className="sidebar-icon">🌍</span>
              Domains
            </NavLink>
          </>
        )}
      </nav>
    </aside>
  );
}

function Header() {
  const { user, logout } = useAuth();
  const { currentDomain } = useDomain();

  return (
    <header className="header">
      <div className="header-title">
        <h1>{currentDomain ? currentDomain.name : 'Hibana Stack'}</h1>
      </div>
      <div className="header-actions">
        <span className="user-name">{user?.username}</span>
        <button onClick={logout} className="btn btn-secondary btn-sm">
          Logout
        </button>
      </div>
    </header>
  );
}

function Layout({ children }) {
  return (
    <div className="app-layout">
      <Sidebar />
      <div className="main-wrapper">
        <Header />
        <main className="main-content">
          {children}
        </main>
      </div>
    </div>
  );
}

// Guard for domain-specific routes
function DomainRoute({ children }) {
  const { currentDomain, loading } = useDomain();

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  // Redirect to domains if no domain selected
  if (!currentDomain) {
    return <Navigate to="/domains" />;
  }

  return children;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={<Navigate to="/domains" />}
      />
      <Route
        path="/dns-providers"
        element={
          <PrivateRoute>
            <Layout>
              <DNSProviders />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/domains"
        element={
          <PrivateRoute>
            <Layout>
              <Domains />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/services"
        element={
          <PrivateRoute>
            <DomainRoute>
              <Layout>
                <Services />
              </Layout>
            </DomainRoute>
          </PrivateRoute>
        }
      />
      <Route
        path="/emails"
        element={
          <PrivateRoute>
            <DomainRoute>
              <Layout>
                <Emails />
              </Layout>
            </DomainRoute>
          </PrivateRoute>
        }
      />
      <Route
        path="/ssh-keys"
        element={
          <PrivateRoute>
            <DomainRoute>
              <Layout>
                <SSHKeys />
              </Layout>
            </DomainRoute>
          </PrivateRoute>
        }
      />
      <Route
        path="/dns"
        element={
          <PrivateRoute>
            <DomainRoute>
              <Layout>
                <DNS />
              </Layout>
            </DomainRoute>
          </PrivateRoute>
        }
      />
      <Route path="*" element={<Navigate to="/domains" />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <DomainProvider>
          <AppRoutes />
        </DomainProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
