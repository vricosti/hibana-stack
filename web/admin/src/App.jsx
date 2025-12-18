import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { DomainProvider, useDomain } from './context/DomainContext';
import Login from './pages/Login';
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

function DomainSelector() {
  const { domains, currentDomain, selectDomain, loading } = useDomain();

  if (loading) {
    return <div className="domain-selector loading">Loading...</div>;
  }

  if (domains.length === 0) {
    return <div className="domain-selector empty">No domains</div>;
  }

  return (
    <div className="domain-selector">
      <select
        value={currentDomain?.id || ''}
        onChange={(e) => {
          const domain = domains.find(d => d.id === parseInt(e.target.value));
          if (domain) selectDomain(domain);
        }}
      >
        {domains.map(domain => (
          <option key={domain.id} value={domain.id}>
            {domain.name}
          </option>
        ))}
      </select>
    </div>
  );
}

function Sidebar() {
  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2>Hibana</h2>
      </div>

      <DomainSelector />

      <nav className="sidebar-nav">
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
        <h1>{currentDomain?.name || 'Hibana Stack'}</h1>
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

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={<Navigate to="/services" />}
      />
      <Route
        path="/services"
        element={
          <PrivateRoute>
            <Layout>
              <Services />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/emails"
        element={
          <PrivateRoute>
            <Layout>
              <Emails />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/ssh-keys"
        element={
          <PrivateRoute>
            <Layout>
              <SSHKeys />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/dns"
        element={
          <PrivateRoute>
            <Layout>
              <DNS />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route path="*" element={<Navigate to="/services" />} />
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
