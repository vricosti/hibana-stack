import React, { useState, useEffect } from 'react';
import { dnsAPI, domainAPI } from '../services/api';
import DNSModal from '../components/DNSModal';

export default function DNS() {
  const [dnsRecords, setDnsRecords] = useState([]);
  const [domains, setDomains] = useState([]);
  const [selectedDomain, setSelectedDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingRecord, setEditingRecord] = useState(null);

  useEffect(() => {
    loadDomains();
  }, []);

  useEffect(() => {
    if (selectedDomain) {
      loadDNSRecords();
    }
  }, [selectedDomain]);

  const loadDomains = async () => {
    try {
      const response = await domainAPI.getAll();
      const domainList = response.data.data || [];
      setDomains(domainList);
      if (domainList.length > 0) {
        setSelectedDomain(domainList[0].id);
      }
    } catch (err) {
      setError('Failed to load domains');
    }
  };

  const loadDNSRecords = async () => {
    if (!selectedDomain) return;

    try {
      setLoading(true);
      const response = await dnsAPI.getByDomain(selectedDomain);
      setDnsRecords(response.data.data || []);
    } catch (err) {
      setError('Failed to load DNS records');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingRecord(null);
    setShowModal(true);
  };

  const handleEdit = (record) => {
    setEditingRecord(record);
    setShowModal(true);
  };

  const handleDelete = async (record) => {
    if (!window.confirm(`Delete DNS record ${record.name} (${record.type})?`)) return;

    try {
      await dnsAPI.delete(record.id);
      loadDNSRecords();
    } catch (err) {
      alert('Failed to delete DNS record');
    }
  };

  const handleModalClose = (refresh) => {
    setShowModal(false);
    setEditingRecord(null);
    if (refresh) {
      loadDNSRecords();
    }
  };

  // Format DNS name for display: show @ for domain root, otherwise just subdomain
  const formatDNSName = (name) => {
    if (!name) return '@';

    const currentDomain = domains.find(d => d.id === selectedDomain);
    if (!currentDomain) return name;

    // If the name is exactly the domain name, show @
    if (name === currentDomain.name || name === '@') {
      return '@';
    }

    // If the name ends with the domain, show just the subdomain part
    if (name.endsWith('.' + currentDomain.name)) {
      return name.replace('.' + currentDomain.name, '');
    }

    // Otherwise return the name as-is
    return name;
  };

  return (
    <div>
      <div className="page-header">
        <h2>DNS Records</h2>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <select
            className="form-control"
            value={selectedDomain}
            onChange={(e) => setSelectedDomain(e.target.value)}
            style={{ width: '300px' }}
          >
            {domains.map((domain) => (
              <option key={domain.id} value={domain.id}>
                {domain.name}
              </option>
            ))}
          </select>
          <button
            className="btn btn-primary"
            onClick={handleAdd}
            disabled={!selectedDomain}
          >
            Add DNS Record
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="loading"><div className="spinner"></div></div>
      ) : dnsRecords.length > 0 ? (
        <div className="table-container">
          <table className="dns-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Content</th>
                <th>TTL</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {dnsRecords.map((record) => (
                <tr key={record.id}>
                  <td>{formatDNSName(record.name)}</td>
                  <td><span className="dns-type-badge">{record.type}</span></td>
                  <td className="dns-content">{record.content}</td>
                  <td>{record.ttl}{record.priority ? ` (Pri: ${record.priority})` : ''}</td>
                  <td>
                    <div className="table-actions">
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={() => handleEdit(record)}
                      >
                        Edit
                      </button>
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={() => handleDelete(record)}
                      >
                        Delete
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
          <div className="empty-state-icon">🌐</div>
          <p>No DNS records yet. Add your first DNS record!</p>
        </div>
      )}

      {showModal && (
        <DNSModal
          record={editingRecord}
          domainId={selectedDomain}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
