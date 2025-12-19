import React, { useState, useEffect } from 'react';
import { domainAPI, dnsProviderAPI } from '../services/api';

export default function DomainModal({ domain, onClose, dnsProviders = [] }) {
  const [selectedProvider, setSelectedProvider] = useState(dnsProviders[0]?.id || '');
  const [availableDomains, setAvailableDomains] = useState([]);
  const [loadingDomains, setLoadingDomains] = useState(false);
  const [formData, setFormData] = useState({
    name: domain?.name || '',
    server_ip: domain?.server_ip || ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [isPrepared, setIsPrepared] = useState(null);
  const [checkingPrepared, setCheckingPrepared] = useState(false);

  // Load available domains when provider changes
  useEffect(() => {
    if (selectedProvider && !domain) {
      loadAvailableDomains(selectedProvider);
    }
  }, [selectedProvider, domain]);

  // Check if domain is prepared when domain name changes
  useEffect(() => {
    if (formData.name && !domain) {
      checkDomainPrepared(formData.name);
    } else if (domain) {
      // Existing domain is always considered prepared
      setIsPrepared(true);
    }
  }, [formData.name, domain]);

  const loadAvailableDomains = async (providerId) => {
    setLoadingDomains(true);
    try {
      const domains = await dnsProviderAPI.getAvailableDomains(providerId);
      // Filter out managed domains (already added to the server)
      const notManaged = (domains || []).filter(d => !d.managed);
      setAvailableDomains(notManaged);
      // Auto-select first available domain if exists
      const firstAvailable = notManaged.find(d => d.available);
      if (firstAvailable && !formData.name) {
        setFormData(prev => ({ ...prev, name: firstAvailable.domain }));
      } else if (notManaged.length > 0 && !formData.name) {
        setFormData(prev => ({ ...prev, name: notManaged[0].domain }));
      }
    } catch (err) {
      console.error('Failed to load available domains:', err);
      setAvailableDomains([]);
    } finally {
      setLoadingDomains(false);
    }
  };

  const checkDomainPrepared = async (domainName) => {
    setCheckingPrepared(true);
    try {
      const result = await domainAPI.checkPrepared(domainName);
      setIsPrepared(result.prepared);
    } catch (err) {
      console.error('Failed to check domain preparation:', err);
      setIsPrepared(false);
    } finally {
      setCheckingPrepared(false);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      if (domain) {
        // Update
        await domainAPI.update(domain.id, {
          server_ip: formData.server_ip
        });
      } else {
        // Create
        await domainAPI.create(formData);
      }
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to save domain');
    } finally {
      setLoading(false);
    }
  };

  const currentProvider = dnsProviders.find(p => p.id === parseInt(selectedProvider));
  const isExternalProvider = currentProvider && currentProvider.type === 'external';

  // Determine if Save should be disabled
  const canSave = domain || (isPrepared === true && formData.name);

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{domain ? 'Edit Domain' : 'Add Domain'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>&times;</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            {!domain && dnsProviders.length > 1 && (
              <div className="form-group">
                <label>DNS Provider</label>
                <select
                  className="form-control"
                  value={selectedProvider}
                  onChange={(e) => {
                    setSelectedProvider(e.target.value);
                    setFormData(prev => ({ ...prev, name: '' }));
                    setIsPrepared(null);
                  }}
                >
                  {dnsProviders.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              </div>
            )}

            <div className="form-group">
              <label>Domain Name *</label>
              {!domain && isExternalProvider ? (
                loadingDomains ? (
                  <div className="form-control" style={{ color: '#666' }}>Loading domains...</div>
                ) : availableDomains.length > 0 ? (
                  <select
                    className="form-control"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    required
                  >
                    <option value="">Select a domain...</option>
                    {availableDomains.map(d => (
                      <option key={d.domain} value={d.domain}>
                        {d.domain} ({d.available ? 'Available' : 'External'})
                      </option>
                    ))}
                  </select>
                ) : (
                  <div>
                    <input
                      type="text"
                      className="form-control"
                      value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      required
                      placeholder="example.com"
                    />
                    <small className="text-warning">
                      No domains found from provider. Enter domain name manually.
                    </small>
                  </div>
                )
              ) : (
                <input
                  type="text"
                  className="form-control"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                  disabled={!!domain}
                  placeholder="example.com"
                />
              )}
            </div>

            {/* Domain preparation status */}
            {!domain && formData.name && (
              <div className="form-group">
                {checkingPrepared ? (
                  <div style={{ color: '#666', fontSize: '0.9em' }}>
                    Checking domain preparation...
                  </div>
                ) : isPrepared === false ? (
                  <div className="alert alert-warning" style={{ marginBottom: 0 }}>
                    <strong>Domain not prepared</strong>
                    <p style={{ margin: '8px 0 0 0', fontSize: '0.9em' }}>
                      Connect to the server via SSH and run:
                    </p>
                    <code style={{
                      display: 'block',
                      marginTop: '8px',
                      padding: '8px 12px',
                      background: '#2d2d2d',
                      color: '#f8f8f2',
                      borderRadius: '4px',
                      fontSize: '0.85em'
                    }}>
                      sudo hibana add domain {formData.name}
                    </code>
                  </div>
                ) : isPrepared === true ? (
                  <div style={{ color: '#28a745', fontSize: '0.9em', display: 'flex', alignItems: 'center', gap: '5px' }}>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
                    </svg>
                    Domain is prepared and ready to be added
                  </div>
                ) : null}
              </div>
            )}

            <div className="form-group">
              <label>Server IP</label>
              <input
                type="text"
                className="form-control"
                value={formData.server_ip}
                onChange={(e) => setFormData({ ...formData, server_ip: e.target.value })}
                placeholder="Leave empty to use default"
              />
            </div>

          </div>

          <div className="modal-footer">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => onClose(false)}
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading || !canSave || checkingPrepared}
              title={!canSave ? 'Domain must be prepared first' : ''}
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
