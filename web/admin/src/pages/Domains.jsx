import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { domainAPI } from '../services/api';
import DomainModal from '../components/DomainModal';

export default function Domains() {
  const navigate = useNavigate();
  const [domains, setDomains] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingDomain, setEditingDomain] = useState(null);

  useEffect(() => {
    loadDomains();
  }, []);

  const loadDomains = async () => {
    try {
      setLoading(true);
      const response = await domainAPI.getAll();
      setDomains(response.data.data || []);
    } catch (err) {
      setError('Failed to load domains');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingDomain(null);
    setShowModal(true);
  };

  const handleEdit = (domain) => {
    setEditingDomain(domain);
    setShowModal(true);
  };

  const handleDelete = async (domain) => {
    if (!window.confirm(`Delete domain ${domain.name}?`)) return;

    try {
      await domainAPI.delete(domain.id);
      loadDomains();
    } catch (err) {
      alert('Failed to delete domain');
    }
  };

  const handleModalClose = (refresh) => {
    setShowModal(false);
    setEditingDomain(null);
    if (refresh) {
      loadDomains();
    }
  };

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>Domains</h2>
        <button className="btn btn-primary" onClick={handleAdd}>
          Add Domain
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {domains.length > 0 ? (
        <div className="data-table">
          {domains.map((domain) => (
            <div key={domain.id} className="data-item">
              <div className="data-item-info">
                <h4>{domain.name}</h4>
                <p>Server IP: {domain.server_ip}</p>
                <p>Created: {new Date(domain.created_at).toLocaleDateString()}</p>
              </div>
              <div className="data-item-actions">
                {domain.username && (
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={() => navigate(`/domains/${domain.name}/ssh-keys`)}
                    title="Manage SSH keys"
                  >
                    🔑 SSH Keys
                  </button>
                )}
                <button
                  className="btn btn-secondary"
                  onClick={() => handleEdit(domain)}
                >
                  Edit
                </button>
                <button
                  className="btn btn-danger"
                  onClick={() => handleDelete(domain)}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="empty-state">
          <div className="empty-state-icon">🌐</div>
          <p>No domains yet. Add your first domain to get started!</p>
        </div>
      )}

      {showModal && (
        <DomainModal
          domain={editingDomain}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
