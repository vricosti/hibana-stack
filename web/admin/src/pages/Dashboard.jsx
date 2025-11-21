import React, { useState, useEffect } from 'react';
import { domainAPI } from '../services/api';

export default function Dashboard() {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    try {
      setLoading(true);
      const response = await domainAPI.getStats();
      setStats(response.data.data);
    } catch (err) {
      setError('Failed to load statistics');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  if (error) {
    return <div className="alert alert-error">{error}</div>;
  }

  return (
    <div>
      <h2>Dashboard</h2>

      <div className="stats-grid">
        <div className="stat-card">
          <h3>Total Domains</h3>
          <p className="stat-value">{stats?.total_domains || 0}</p>
        </div>
        <div className="stat-card">
          <h3>Email Accounts</h3>
          <p className="stat-value">{stats?.total_email_accounts || 0}</p>
        </div>
        <div className="stat-card">
          <h3>DNS Records</h3>
          <p className="stat-value">{stats?.total_dns_records || 0}</p>
        </div>
      </div>

      <div className="card">
        <h3>Domain Statistics</h3>
        {stats?.domain_stats && stats.domain_stats.length > 0 ? (
          <div className="data-table">
            {stats.domain_stats.map((domain) => (
              <div key={domain.domain_name} className="data-item">
                <div className="data-item-info">
                  <h4>{domain.domain_name}</h4>
                  <p>
                    {domain.email_count} email accounts • {domain.dns_record_count} DNS records
                  </p>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="empty-state">
            <p>No domains yet</p>
          </div>
        )}
      </div>
    </div>
  );
}
