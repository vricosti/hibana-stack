import React, { useState } from 'react';
import { dnsProviderAPI } from '../services/api';

const PROVIDERS = [
  { value: 'hostinger', label: 'Hostinger', requiresToken: true },
  { value: 'cloudflare', label: 'Cloudflare', requiresToken: true },
  { value: 'ovhcloud', label: 'OVHcloud', requiresOVH: true }
];

export default function DNSProviderModal({ provider, onClose }) {
  const isEditing = !!provider;

  const [formData, setFormData] = useState({
    name: provider?.name || '',
    type: provider?.type || 'external',
    provider: provider?.provider || '',
    api_token: provider?.api_token || '',
    endpoint: provider?.endpoint || 'ovh-eu',
    application_key: provider?.application_key || '',
    application_secret: provider?.application_secret || '',
    consumer_key: provider?.consumer_key || ''
  });
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');
  const [testSuccess, setTestSuccess] = useState(false);

  const selectedProvider = PROVIDERS.find(p => p.value === formData.provider);

  const handleProviderChange = (provider) => {
    setFormData({
      ...formData,
      provider,
      type: 'external',
      api_token: '',
      application_key: '',
      application_secret: '',
      consumer_key: ''
    });
    setTestSuccess(false);
    setError('');
  };

  const handleTestConnection = async () => {
    setTesting(true);
    setError('');
    setTestSuccess(false);

    try {
      await dnsProviderAPI.testConnection(formData);
      setTestSuccess(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Connection test failed');
    } finally {
      setTesting(false);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');

    if (!formData.name.trim()) {
      setError('Name is required');
      return;
    }
    if (!formData.provider) {
      setError('Please select a provider');
      return;
    }

    setLoading(true);
    try {
      if (isEditing) {
        await dnsProviderAPI.update(provider.id, formData);
      } else {
        await dnsProviderAPI.create(formData);
      }
      onClose(true);
    } catch (err) {
      const action = isEditing ? 'update' : 'add';
      setError(err.response?.data?.error || `Failed to ${action} DNS provider`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{isEditing ? 'Edit DNS Provider' : 'Add DNS Provider'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>&times;</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}
            {testSuccess && <div className="alert alert-success">Connection successful!</div>}

            <div className="form-group">
              <label>Name *</label>
              <input
                type="text"
                className="form-control"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="My DNS Provider"
              />
            </div>

            <div className="form-group">
              <label>Provider *</label>
              <select
                className="form-control"
                value={formData.provider}
                onChange={(e) => handleProviderChange(e.target.value)}
                disabled={isEditing}
              >
                <option value="">Select a provider...</option>
                {PROVIDERS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
              {isEditing && <small style={{ color: '#666' }}>Provider type cannot be changed after creation</small>}
            </div>

            {selectedProvider?.requiresToken && (
              <div className="form-group">
                <label>API Token *</label>
                <input
                  type="password"
                  className="form-control"
                  value={formData.api_token}
                  onChange={(e) => setFormData({ ...formData, api_token: e.target.value })}
                  placeholder="Enter API token"
                />
                <small>
                  {formData.provider === 'hostinger' && (
                    <a href="https://hpanel.hostinger.com/profile/api" target="_blank" rel="noopener noreferrer">
                      Get your Hostinger API token
                    </a>
                  )}
                  {formData.provider === 'cloudflare' && (
                    <a href="https://dash.cloudflare.com/profile/api-tokens" target="_blank" rel="noopener noreferrer">
                      Get your Cloudflare API token
                    </a>
                  )}
                </small>
              </div>
            )}

            {selectedProvider?.requiresOVH && (
              <>
                <div className="form-group">
                  <label>Endpoint</label>
                  <select
                    className="form-control"
                    value={formData.endpoint}
                    onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
                  >
                    <option value="ovh-eu">OVH Europe</option>
                    <option value="ovh-ca">OVH Canada</option>
                    <option value="ovh-us">OVH US</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Application Key *</label>
                  <input
                    type="text"
                    className="form-control"
                    value={formData.application_key}
                    onChange={(e) => setFormData({ ...formData, application_key: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label>Application Secret *</label>
                  <input
                    type="password"
                    className="form-control"
                    value={formData.application_secret}
                    onChange={(e) => setFormData({ ...formData, application_secret: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label>Consumer Key *</label>
                  <input
                    type="password"
                    className="form-control"
                    value={formData.consumer_key}
                    onChange={(e) => setFormData({ ...formData, consumer_key: e.target.value })}
                  />
                </div>
                <small>
                  <a href="https://eu.api.ovh.com/createToken/" target="_blank" rel="noopener noreferrer">
                    Create OVH API credentials
                  </a>
                </small>
              </>
            )}

            {formData.provider && (
              <div style={{ marginTop: '15px' }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={handleTestConnection}
                  disabled={testing}
                >
                  {testing ? 'Testing...' : 'Test Connection'}
                </button>
              </div>
            )}
          </div>

          <div className="modal-footer">
            <button type="button" className="btn btn-secondary" onClick={() => onClose(false)} disabled={loading}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? (isEditing ? 'Updating...' : 'Adding...') : (isEditing ? 'Update Provider' : 'Add Provider')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
