import React, { useState } from 'react';
import { dnsAPI } from '../services/api';

const DNS_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA'];

export default function DNSModal({ record, domainId, onClose }) {
  const [formData, setFormData] = useState({
    name: record?.name || '',
    type: record?.type || 'A',
    content: record?.content || '',
    ttl: record?.ttl || 3600,
    priority: record?.priority || 0
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const needsPriority = formData.type === 'MX' || formData.type === 'SRV';

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const data = {
        name: formData.name,
        type: formData.type,
        content: formData.content,
        ttl: parseInt(formData.ttl)
      };

      if (needsPriority) {
        data.priority = parseInt(formData.priority);
      }

      if (record) {
        // Update
        await dnsAPI.update(record.id, {
          content: data.content,
          ttl: data.ttl,
          ...(needsPriority && { priority: data.priority })
        });
      } else {
        // Create
        await dnsAPI.create(domainId, data);
      }
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to save DNS record');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{record ? 'Edit DNS Record' : 'Add DNS Record'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <div className="form-group">
              <label>Name *</label>
              <input
                type="text"
                className="form-control"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
                disabled={!!record}
                placeholder="@ or subdomain"
              />
              <small style={{color: '#666'}}>Use @ for root domain</small>
            </div>

            <div className="form-group">
              <label>Type *</label>
              <select
                className="form-control"
                value={formData.type}
                onChange={(e) => setFormData({ ...formData, type: e.target.value })}
                required
                disabled={!!record}
              >
                {DNS_TYPES.map((type) => (
                  <option key={type} value={type}>{type}</option>
                ))}
              </select>
            </div>

            <div className="form-group">
              <label>Content *</label>
              <input
                type="text"
                className="form-control"
                value={formData.content}
                onChange={(e) => setFormData({ ...formData, content: e.target.value })}
                required
                placeholder={
                  formData.type === 'A' ? '192.168.1.1' :
                  formData.type === 'CNAME' ? 'example.com' :
                  formData.type === 'MX' ? 'mail.example.com' :
                  formData.type === 'TXT' ? '"v=spf1 ..."' : ''
                }
              />
            </div>

            <div className="form-group">
              <label>TTL (seconds) *</label>
              <input
                type="number"
                className="form-control"
                value={formData.ttl}
                onChange={(e) => setFormData({ ...formData, ttl: e.target.value })}
                required
                min="60"
              />
            </div>

            {needsPriority && (
              <div className="form-group">
                <label>Priority *</label>
                <input
                  type="number"
                  className="form-control"
                  value={formData.priority}
                  onChange={(e) => setFormData({ ...formData, priority: e.target.value })}
                  required
                  min="0"
                />
              </div>
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
