import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDomain } from '../context/DomainContext';
import { domainAPI } from '../services/api';

export default function Domains() {
  const navigate = useNavigate();
  const { domains, loading, refreshDomains, selectDomain } = useDomain();
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState(null);

  const handleSelectDomain = (domain) => {
    selectDomain(domain);
    navigate('/services');
  };

  const handleDeleteDomain = async (e, domain) => {
    e.stopPropagation();

    // Check if it's the primary domain (last in list = first created)
    if (domains.length > 0 && domain.id === domains[domains.length - 1].id) {
      alert('Cannot delete the primary domain');
      return;
    }

    if (!window.confirm(`Delete domain ${domain.name}? This will remove all associated data.`)) {
      return;
    }

    setDeleting(domain.id);
    try {
      await domainAPI.delete(domain.id);
      refreshDomains();
    } catch (err) {
      setError(`Failed to delete domain: ${err.response?.data?.error || err.message}`);
    } finally {
      setDeleting(null);
    }
  };

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  // Determine primary domain (last in list = first created)
  const primaryDomainId = domains.length > 0 ? domains[domains.length - 1].id : null;

  return (
    <div>
      <div className="page-header">
        <h2>Domains</h2>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {domains.length > 0 ? (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                <th>Domain</th>
                <th>Server IP</th>
                <th>User</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {domains.map((domain) => (
                <tr
                  key={domain.id}
                  onClick={() => handleSelectDomain(domain)}
                  className="clickable-row"
                >
                  <td>
                    <strong>{domain.display_name || domain.name}</strong>
                    {domain.id === primaryDomainId && (
                      <span className="badge badge-primary" style={{ marginLeft: '8px' }}>Primary</span>
                    )}
                  </td>
                  <td>{domain.server_ip || '-'}</td>
                  <td>{domain.username || '-'}</td>
                  <td>{new Date(domain.created_at).toLocaleDateString()}</td>
                  <td>
                    <div className="table-actions">
                      {domain.id !== primaryDomainId && (
                        <button
                          className="btn-icon btn-icon-danger"
                          onClick={(e) => handleDeleteDomain(e, domain)}
                          disabled={deleting === domain.id}
                          title="Delete"
                        >
                          {deleting === domain.id ? '...' : (
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                              <polyline points="3 6 5 6 21 6"/>
                              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                              <line x1="10" y1="11" x2="10" y2="17"/>
                              <line x1="14" y1="11" x2="14" y2="17"/>
                            </svg>
                          )}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="empty-state">
          <p>No domains configured. Use <code>hibana init</code> to add domains.</p>
        </div>
      )}
    </div>
  );
}
