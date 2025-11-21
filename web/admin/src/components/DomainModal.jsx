import React, { useState } from 'react';
import { domainAPI } from '../services/api';

export default function DomainModal({ domain, onClose }) {
  const [formData, setFormData] = useState({
    name: domain?.name || '',
    server_ip: domain?.server_ip || '',
    create_user: !domain,
    ssh_key_mode: 'auto',
    ssh_public_key: ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

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

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{domain ? 'Edit Domain' : 'Add Domain'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <div className="form-group">
              <label>Domain Name *</label>
              <input
                type="text"
                className="form-control"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
                disabled={!!domain}
                placeholder="example.com"
              />
            </div>

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

            {!domain && (
              <>
                <div className="form-group">
                  <div className="form-check">
                    <input
                      type="checkbox"
                      id="create_user"
                      checked={formData.create_user}
                      onChange={(e) => setFormData({ ...formData, create_user: e.target.checked })}
                    />
                    <label htmlFor="create_user">Create domain user</label>
                  </div>
                </div>

                {formData.create_user && (
                  <>
                    <div className="form-group">
                      <label>SSH Key Mode</label>
                      <select
                        className="form-control"
                        value={formData.ssh_key_mode}
                        onChange={(e) => setFormData({ ...formData, ssh_key_mode: e.target.value })}
                      >
                        <option value="auto">Auto (generate key)</option>
                        <option value="manual">Manual (provide key)</option>
                      </select>
                    </div>

                    {formData.ssh_key_mode === 'manual' && (
                      <div className="form-group">
                        <label>SSH Public Key *</label>
                        <textarea
                          className="form-control"
                          rows="3"
                          value={formData.ssh_public_key}
                          onChange={(e) => setFormData({ ...formData, ssh_public_key: e.target.value })}
                          required={formData.create_user && formData.ssh_key_mode === 'manual'}
                          placeholder="ssh-ed25519 AAAAC3NzaC1..."
                        />
                      </div>
                    )}
                  </>
                )}
              </>
            )}
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
