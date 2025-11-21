import React, { useState } from 'react';
import { emailAPI } from '../services/api';

export default function EmailModal({ email, domains, onClose }) {
  const [formData, setFormData] = useState({
    domain_id: email?.domain_id || (domains[0]?.id || ''),
    username: email?.username || '',
    password: '',
    full_name: email?.full_name || ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      if (email) {
        // Update - only send fields that changed
        const updateData = {};
        if (formData.password) updateData.password = formData.password;
        if (formData.full_name !== email.full_name) updateData.full_name = formData.full_name;

        if (Object.keys(updateData).length > 0) {
          await emailAPI.update(email.id, updateData);
        }
      } else {
        // Create
        await emailAPI.create(formData.domain_id, {
          username: formData.username,
          password: formData.password,
          full_name: formData.full_name
        });
      }
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to save email account');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{email ? 'Edit Email Account' : 'Add Email Account'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <div className="form-group">
              <label>Domain *</label>
              <select
                className="form-control"
                value={formData.domain_id}
                onChange={(e) => setFormData({ ...formData, domain_id: e.target.value })}
                required
                disabled={!!email}
              >
                {domains.map((domain) => (
                  <option key={domain.id} value={domain.id}>
                    {domain.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="form-group">
              <label>Username *</label>
              <input
                type="text"
                className="form-control"
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                required
                disabled={!!email}
                placeholder="admin"
              />
            </div>

            <div className="form-group">
              <label>Password {!email && '*'}</label>
              <input
                type="password"
                className="form-control"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                required={!email}
                placeholder={email ? 'Leave empty to keep current' : ''}
                minLength="8"
              />
              {!email && <small style={{color: '#666'}}>Minimum 8 characters</small>}
            </div>

            <div className="form-group">
              <label>Full Name</label>
              <input
                type="text"
                className="form-control"
                value={formData.full_name}
                onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                placeholder="John Doe"
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
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
