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
