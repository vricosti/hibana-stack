import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDomain } from '../context/DomainContext';
import { domainAPI, dnsProviderAPI } from '../services/api';
import DomainModal from '../components/DomainModal';
import DNSProviderModal from '../components/DNSProviderModal';

const formatProviderName = (name) => {
  const names = {
    powerdns: 'PowerDNS',
    hostinger: 'Hostinger',
    cloudflare: 'Cloudflare',
    ovh: 'OVH'
  };
  return names[name] || name;
};

export default function Domains() {
  const navigate = useNavigate();
  const { domains, loading, refreshDomains, selectDomain } = useDomain();
  const [showDomainModal, setShowDomainModal] = useState(false);
  const [showProviderModal, setShowProviderModal] = useState(false);
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState(null);
  const [deletingProvider, setDeletingProvider] = useState(null);
  const [dnsProviders, setDnsProviders] = useState([]);
  const [loadingProviders, setLoadingProviders] = useState(true);

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoadingProviders(true);
      const providers = await dnsProviderAPI.getAll();
      setDnsProviders(providers || []);
    } catch (err) {
      console.error('Failed to load DNS providers:', err);
    } finally {
      setLoadingProviders(false);
    }
  };

  const handleAddDomain = () => {
    if (dnsProviders.length === 0) {
      setError('Please add a DNS provider first');
      return;
    }
    setShowDomainModal(true);
  };

  const handleDomainModalClose = (refresh) => {
    setShowDomainModal(false);
    if (refresh) {
      refreshDomains();
    }
  };

  const handleProviderModalClose = (refresh) => {
    setShowProviderModal(false);
    if (refresh) {
      loadProviders();
    }
  };

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

  if (loading || loadingProviders) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  // Determine primary domain (last in list = first created)
  const primaryDomainId = domains.length > 0 ? domains[domains.length - 1].id : null;

  return (
    <div>
      <div className="page-header">
        <h2>Domains & DNS Providers</h2>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {/* DNS Providers Section */}
      <div className="section-header">
        <h3>DNS Providers</h3>
        <button className="btn btn-primary btn-sm" onClick={() => setShowProviderModal(true)}>
          Add Provider
        </button>
      </div>

      {dnsProviders.length > 0 ? (
        <div className="table-container" style={{ marginBottom: '25px' }}>
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
        <div className="empty-state" style={{ marginBottom: '25px' }}>
          <p>No DNS providers configured. Add a provider to manage your domains.</p>
        </div>
      )}

      {/* Domains Section */}
      <div className="section-header">
        <h3>Domains</h3>
        <button className="btn btn-primary btn-sm" onClick={handleAddDomain}>
          Add Domain
        </button>
      </div>

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
                    <strong>{domain.name}</strong>
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
          <p>No domains configured. Add your first domain to get started.</p>
        </div>
      )}

      {showDomainModal && (
        <DomainModal
          onClose={handleDomainModalClose}
          dnsProviders={dnsProviders}
        />
      )}

      {showProviderModal && (
        <DNSProviderModal onClose={handleProviderModalClose} />
      )}
    </div>
  );
}
