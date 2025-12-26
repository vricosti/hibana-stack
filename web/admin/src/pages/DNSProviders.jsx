import React, { useState, useEffect } from 'react';
import { dnsProviderAPI } from '../services/api';
import DNSProviderModal from '../components/DNSProviderModal';

const formatProviderName = (name) => {
  const names = {
    powerdns: 'PowerDNS',
    hostinger: 'Hostinger',
    cloudflare: 'Cloudflare',
    ovhcloud: 'OVHcloud'
  };
  return names[name] || name;
};

export default function DNSProviders() {
  const [showProviderModal, setShowProviderModal] = useState(false);
  const [editingProvider, setEditingProvider] = useState(null);
  const [error, setError] = useState('');
  const [deletingProvider, setDeletingProvider] = useState(null);
  const [dnsProviders, setDnsProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [availableDomains, setAvailableDomains] = useState([]);
  const [loadingDomains, setLoadingDomains] = useState(false);

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoading(true);
      const providers = await dnsProviderAPI.getAll();
      setDnsProviders(providers || []);

      // Load available domains for the first provider
      if (providers && providers.length > 0) {
        loadAvailableDomains(providers[0].id);
      }
    } catch (err) {
      console.error('Failed to load DNS providers:', err);
      // Display detailed error message from API response
      const errorMessage = err.response?.data?.error || err.message || 'Failed to load DNS providers';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const loadAvailableDomains = async (providerId) => {
    try {
      setLoadingDomains(true);
      const domains = await dnsProviderAPI.getAvailableDomains(providerId);
      setAvailableDomains(domains || []);
    } catch (err) {
      console.error('Failed to load available domains:', err);
      // Display detailed error message from API response
      const errorMessage = err.response?.data?.error || err.message || 'Failed to load available domains';
      setError(errorMessage);
    } finally {
      setLoadingDomains(false);
    }
  };

  const handleAddProvider = () => {
    setEditingProvider(null);
    setShowProviderModal(true);
  };

  const handleEditProvider = (provider) => {
    setEditingProvider(provider);
    setShowProviderModal(true);
  };

  const handleProviderModalClose = (refresh) => {
    setShowProviderModal(false);
    setEditingProvider(null);
    if (refresh) {
      loadProviders();
    }
  };

  const handleDeleteProvider = async (e, provider) => {
    e.stopPropagation();

    if (!window.confirm(`Delete DNS provider "${provider.name}"?`)) {
      return;
    }

    setDeletingProvider(provider.id);
    try {
      await dnsProviderAPI.delete(provider.id);
      loadProviders();
    } catch (err) {
      setError(`Failed to delete provider: ${err.response?.data?.error || err.message}`);
    } finally {
      setDeletingProvider(null);
    }
  };

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>DNS Providers</h2>
        <button className="btn btn-primary" onClick={handleAddProvider}>
          Add Provider
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {dnsProviders.length > 0 ? (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Provider</th>
                <th>Type</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {dnsProviders.map((provider) => (
                <tr key={provider.id}>
                  <td><strong>{provider.name}</strong></td>
                  <td>
                    <span className={`dns-provider-badge dns-provider-${provider.provider}`}>
                      {formatProviderName(provider.provider)}
                    </span>
                  </td>
                  <td>{provider.type === 'local' ? 'Local' : 'External'}</td>
                  <td>{new Date(provider.created_at).toLocaleDateString()}</td>
                  <td>
                    <div className="table-actions">
                      <button
                        className="btn-icon"
                        onClick={(e) => { e.stopPropagation(); handleEditProvider(provider); }}
                        title="Edit"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                      </button>
                      <button
                        className="btn-icon btn-icon-danger"
                        onClick={(e) => handleDeleteProvider(e, provider)}
                        disabled={deletingProvider === provider.id}
                        title="Delete"
                      >
                        {deletingProvider === provider.id ? '...' : (
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                            <polyline points="3 6 5 6 21 6"/>
                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                            <line x1="10" y1="11" x2="10" y2="17"/>
                            <line x1="14" y1="11" x2="14" y2="17"/>
                          </svg>
                        )}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="empty-state">
          <p>No DNS providers configured. Add a provider to manage your domains.</p>
        </div>
      )}

      {/* Available Domains Section */}
      {dnsProviders.length > 0 && (
        <>
          <div className="section-header" style={{ marginTop: '30px' }}>
            <h3>Available Domains</h3>
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => loadAvailableDomains(dnsProviders[0].id)}
              disabled={loadingDomains}
            >
              {loadingDomains ? 'Loading...' : 'Refresh'}
            </button>
          </div>

          {loadingDomains ? (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '40px',
              gap: '10px'
            }}>
              <div className="spinner"></div>
              <span style={{ color: '#666' }}>Loading domains from Hostinger...</span>
            </div>
          ) : availableDomains.length > 0 ? (
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Domain</th>
                    <th>Target</th>
                    <th>Server</th>
                    <th>Type</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {availableDomains.map((domain) => (
                    <tr key={domain.domain}>
                      <td><strong>{domain.domain}</strong></td>
                      <td>
                        <code style={{ fontSize: '0.9em', color: '#666' }}>
                          {domain.target || '-'}
                        </code>
                      </td>
                      <td>
                        {domain.managed ? (
                          <span style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '5px',
                            color: '#28a745',
                            fontWeight: '500'
                          }}>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                              <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
                            </svg>
                            Managed
                          </span>
                        ) : domain.available ? (
                          <span style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '5px',
                            color: '#17a2b8',
                            fontWeight: '500'
                          }}>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                              <circle cx="12" cy="12" r="10"/>
                            </svg>
                            Available
                          </span>
                        ) : (
                          <span style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '5px',
                            color: '#dc3545',
                            fontWeight: '500'
                          }}>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                              <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                            </svg>
                            External
                          </span>
                        )}
                      </td>
                      <td>
                        {domain.record_type && (
                          <span className="badge" style={{
                            background: domain.record_type === 'A' ? '#28a745' : '#17a2b8',
                            color: 'white',
                            padding: '2px 8px',
                            borderRadius: '4px',
                            fontSize: '0.8em'
                          }}>
                            {domain.record_type}
                          </span>
                        )}
                      </td>
                      <td>
                        <span style={{
                          color: domain.status === 'active' ? '#28a745' : '#666',
                          textTransform: 'capitalize'
                        }}>
                          {domain.status || '-'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="empty-state">
              <p>No domains found from this provider.</p>
            </div>
          )}
        </>
      )}

      {showProviderModal && (
        <DNSProviderModal
          provider={editingProvider}
          onClose={handleProviderModalClose}
        />
      )}
    </div>
  );
}
