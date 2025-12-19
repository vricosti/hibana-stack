import React, { useState } from 'react';
import { serviceAPI } from '../services/api';

export default function AddServiceModal({ domainName, onClose }) {
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await serviceAPI.create(domainName, { name: name.toLowerCase() });
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to create service');
    } finally {
      setLoading(false);
    }
  };

  const validateName = (value) => {
    // Only allow lowercase letters, numbers, and hyphens
    return /^[a-z][a-z0-9-]*$/.test(value) || value === '';
  };

  const handleNameChange = (e) => {
    const value = e.target.value.toLowerCase();
    if (validateName(value) || value === '') {
      setName(value);
    }
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Add Service</h3>
          <button className="modal-close" onClick={() => onClose(false)}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <p style={{ marginBottom: '20px', color: '#666' }}>
              Create a new website service for <strong>{domainName}</strong>
            </p>

            <div className="form-group">
              <label>Subdomain Name *</label>
              <div style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
                <input
                  type="text"
                  value={name}
                  onChange={handleNameChange}
                  placeholder="e.g., blog, app, shop"
                  required
                  style={{ flex: 1 }}
                  maxLength={63}
                />
                <span style={{ color: '#666' }}>.{domainName}</span>
              </div>
              <small>Use only lowercase letters, numbers, and hyphens. Must start with a letter.</small>
            </div>

            <div className="alert" style={{ background: '#e3f2fd', color: '#1565c0', border: '1px solid #90caf9' }}>
              <strong>What will be created:</strong>
              <ul style={{ margin: '10px 0 0 0', paddingLeft: '20px' }}>
                <li>Directory structure at /srv/{domainName.replace(/\./g, '-')}/{name || 'subdomain'}/</li>
                <li>Default nginx configuration</li>
                <li>Docker compose file with Traefik routing</li>
                <li>Placeholder index.html page</li>
              </ul>
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
              disabled={loading || !name}
            >
              {loading ? 'Creating...' : 'Create Service'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
