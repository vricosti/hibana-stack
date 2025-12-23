import React, { useState, useEffect } from 'react';
import { useDomain } from '../context/DomainContext';
import { dnsAPI } from '../services/api';
import DNSModal from '../components/DNSModal';

export default function DNS() {
  const { currentDomain } = useDomain();
  const [dnsRecords, setDnsRecords] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingRecord, setEditingRecord] = useState(null);

  useEffect(() => {
    if (currentDomain) {
      loadDNSRecords();
    }
  }, [currentDomain]);

  const loadDNSRecords = async () => {
    if (!currentDomain) return;

    try {
      setLoading(true);
      const response = await dnsAPI.getByDomain(currentDomain.id);
      setDnsRecords(response.data.data || []);
      setError(''); // Clear any previous errors
    } catch (err) {
      // Display detailed error message from API response
      const errorMessage = err.response?.data?.error || err.message || 'Failed to load DNS records';
      setError(errorMessage);
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
    if (!name || !currentDomain) return '@';

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

  if (!currentDomain) {
    return (
      <div className="empty-state">
        <p>Please select a domain to view DNS records</p>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <h2>DNS Records</h2>
        <button
          className="btn btn-primary"
          onClick={handleAdd}
        >
          Add DNS Record
        </button>
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
                        className="btn-icon"
                        onClick={() => handleEdit(record)}
                        title="Edit"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                      </button>
                      <button
                        className="btn-icon btn-icon-danger"
                        onClick={() => handleDelete(record)}
                        title="Delete"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                          <polyline points="3 6 5 6 21 6"/>
                          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                          <line x1="10" y1="11" x2="10" y2="17"/>
                          <line x1="14" y1="11" x2="14" y2="17"/>
                        </svg>
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
          <p>No DNS records for {currentDomain.name}. Add your first DNS record!</p>
        </div>
      )}

      {showModal && (
        <DNSModal
          record={editingRecord}
          domainId={currentDomain.id}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
