import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Domains from './pages/Domains';
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

function Layout({ children }) {
  const { user, logout } = useAuth();

  return (
    <div className="app">
      <header className="header">
        <h1>🔥 Hibana Stack</h1>
        <div className="user-info">
          <span>{user?.username}</span>
          <button onClick={logout} className="btn btn-secondary">
            Logout
          </button>
        </div>
      </header>

      <nav className="nav-tabs">
        <NavLink to="/" className={({ isActive }) => `tab-link ${isActive ? 'active' : ''}`}>
          Dashboard
        </NavLink>
        <NavLink to="/domains" className={({ isActive }) => `tab-link ${isActive ? 'active' : ''}`}>
          Domains
        </NavLink>
        <NavLink to="/emails" className={({ isActive }) => `tab-link ${isActive ? 'active' : ''}`}>
          Email Accounts
        </NavLink>
        <NavLink to="/dns" className={({ isActive }) => `tab-link ${isActive ? 'active' : ''}`}>
          DNS Records
        </NavLink>
      </nav>

      <main className="main-content">
        {children}
      </main>
    </div>
  );
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <Layout>
              <Dashboard />
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
        path="/dns"
        element={
          <PrivateRoute>
            <Layout>
              <DNS />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route
        path="/domains/:domain/ssh-keys"
        element={
          <PrivateRoute>
            <Layout>
              <SSHKeys />
            </Layout>
          </PrivateRoute>
        }
      />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
}
