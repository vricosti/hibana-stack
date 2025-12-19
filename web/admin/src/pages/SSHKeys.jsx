import React, { useState, useEffect } from 'react';
import { useDomain } from '../context/DomainContext';
import { sshKeyAPI } from '../services/api';
import SSHKeyModal from '../components/SSHKeyModal';

export default function SSHKeys() {
  const { currentDomain } = useDomain();
  const [keys, setKeys] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (currentDomain) {
      loadKeys();
    }
  }, [currentDomain]);

  const loadKeys = async () => {
    if (!currentDomain) return;

    try {
      setLoading(true);
      const keysData = await sshKeyAPI.list(currentDomain.name);
      setKeys(keysData.keys || []);
      setError('');
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to load SSH keys');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteKey = async (fingerprint) => {
    if (!confirm('Are you sure you want to remove this SSH key?')) {
      return;
    }

    try {
      await sshKeyAPI.remove(currentDomain.name, fingerprint);
      await loadKeys();
    } catch (err) {
      alert('Failed to remove SSH key: ' + (err.response?.data?.error || err.message));
    }
  };

  const handleModalClose = (reload) => {
    setShowAddModal(false);
    if (reload) {
      loadKeys();
    }
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text).then(() => {
      alert('Copied to clipboard!');
    }).catch(() => {
      alert('Failed to copy to clipboard');
    });
  };

  if (!currentDomain) {
    return (
      <div className="empty-state">
        <p>Please select a domain to view SSH keys</p>
      </div>
    );
  }

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>SSH Keys</h2>
        <button className="btn btn-primary" onClick={() => setShowAddModal(true)}>
          Add SSH Key
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {!currentDomain.username && (
        <div className="alert" style={{ background: '#fff3cd', color: '#856404', border: '1px solid #ffeeba' }}>
          <strong>No domain user configured.</strong>
          <p style={{ marginTop: '5px', marginBottom: 0 }}>
            This domain doesn't have a domain user. SSH key management is only available for domains with domain users.
          </p>
        </div>
      )}

      {keys.length === 0 ? (
        <div className="empty-state">
          <p>No SSH keys configured for {currentDomain.name}.</p>
        </div>
      ) : (
        <div className="table-container">
          <table className="dns-table">
            <thead>
              <tr>
                <th>Label</th>
                <th>Fingerprint</th>
                <th>Public Key</th>
                <th>Added</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id}>
                  <td>
                    <strong>{key.label || 'Unnamed Key'}</strong>
                  </td>
                  <td>
                    <code style={{ fontSize: '0.85em' }}>{key.fingerprint}</code>
                  </td>
                  <td className="dns-content">
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <code>{key.public_key.substring(0, 60)}...</code>
                      <button
                        className="btn btn-sm btn-secondary"
                        onClick={() => copyToClipboard(key.public_key)}
                        title="Copy full key"
                      >
                        Copy
                      </button>
                    </div>
                  </td>
                  <td>
                    {key.created_at ? new Date(key.created_at).toLocaleDateString() : 'N/A'}
                  </td>
                  <td>
                    <div className="table-actions">
                      <button
                        className="btn btn-sm btn-danger"
                        onClick={() => handleDeleteKey(key.fingerprint)}
                      >
                        Remove
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="card" style={{ marginTop: '2rem' }}>
        <h3 style={{ marginBottom: '15px' }}>How to Connect via SSH</h3>
        {currentDomain.username ? (
          <div>
            <p>Once you've added your SSH public key, you can connect to the server:</p>
            <pre style={{
              background: '#f5f5f5',
              padding: '15px',
              borderRadius: '5px',
              overflow: 'auto'
            }}>
              <code>ssh {currentDomain.username}@{currentDomain.server_ip || 'YOUR_SERVER_IP'}</code>
            </pre>
            <p style={{ color: '#666', fontSize: '0.9em', marginTop: '10px' }}>
              You'll have access to <code>/srv/{currentDomain.name.replace(/\./g, '-')}/</code> where you can deploy your applications.
            </p>
            <h4 style={{ marginTop: '20px', marginBottom: '10px' }}>Upload files with SCP or rsync:</h4>
            <pre style={{
              background: '#f5f5f5',
              padding: '15px',
              borderRadius: '5px',
              overflow: 'auto',
              marginBottom: '10px'
            }}>
              <code>scp -r dist/* {currentDomain.username}@{currentDomain.server_ip || 'SERVER_IP'}:/srv/{currentDomain.name.replace(/\./g, '-')}/www/src/</code>
            </pre>
            <pre style={{
              background: '#f5f5f5',
              padding: '15px',
              borderRadius: '5px',
              overflow: 'auto'
            }}>
              <code>rsync -avz --delete dist/ {currentDomain.username}@{currentDomain.server_ip || 'SERVER_IP'}:/srv/{currentDomain.name.replace(/\./g, '-')}/www/src/</code>
            </pre>
          </div>
        ) : (
          <p style={{ color: '#666' }}>Configure a domain user to enable SSH access.</p>
        )}
      </div>

      {showAddModal && (
        <SSHKeyModal
          domainName={currentDomain.name}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
