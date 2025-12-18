import React, { useState } from 'react';
import { aliasAPI } from '../services/api';

export default function AliasModal({ alias, domainId, domainName, onClose }) {
  const [formData, setFormData] = useState({
    source_address: alias?.source_address || '',
    destination: alias?.destination || ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      if (alias) {
        // Update
        await aliasAPI.update(alias.id, {
          destination: formData.destination
        });
      } else {
        // Create
        await aliasAPI.create(domainId, {
          source_address: formData.source_address,
          destination: formData.destination
        });
      }
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to save redirect');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{alias ? 'Edit Redirect' : 'Add Redirect'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>x</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <div className="form-group">
              <label>Domain</label>
              <input
                type="text"
                className="form-control"
                value={domainName}
                disabled
                style={{ background: '#f5f5f5' }}
              />
            </div>

            <div className="form-group">
              <label>Source Address *</label>
              <input
                type="text"
                className="form-control"
                value={formData.source_address}
                onChange={(e) => setFormData({ ...formData, source_address: e.target.value })}
                required
                disabled={!!alias}
                placeholder="contact"
              />
              {!alias && (
                <small style={{ color: '#666' }}>
                  Emails to {formData.source_address || 'source'}@{domainName} will be redirected
                </small>
              )}
            </div>

            <div className="form-group">
              <label>Redirect To *</label>
              <input
                type="email"
                className="form-control"
                value={formData.destination}
                onChange={(e) => setFormData({ ...formData, destination: e.target.value })}
                required
                placeholder="user@example.com"
              />
              <small style={{ color: '#666' }}>
                The email address to forward messages to
              </small>
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
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
