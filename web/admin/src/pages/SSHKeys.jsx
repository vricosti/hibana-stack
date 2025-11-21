import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { sshKeyAPI, domainAPI } from '../services/api';
import SSHKeyModal from '../components/SSHKeyModal';

export default function SSHKeys() {
  const { domain } = useParams();
  const navigate = useNavigate();
  const [keys, setKeys] = useState([]);
  const [domainInfo, setDomainInfo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    loadData();
  }, [domain]);

  const loadData = async () => {
    try {
      setLoading(true);
      // Load domain info
      const domains = await domainAPI.list();
      const foundDomain = domains.find(d => d.name === domain);
      setDomainInfo(foundDomain);

      // Load SSH keys
      const keysData = await sshKeyAPI.list(domain);
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
      await sshKeyAPI.remove(domain, fingerprint);
      await loadData();
    } catch (err) {
      alert('Failed to remove SSH key: ' + (err.response?.data?.error || err.message));
    }
  };

  const handleModalClose = (reload) => {
    setShowAddModal(false);
    if (reload) {
      loadData();
    }
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text).then(() => {
      alert('Copied to clipboard!');
    }).catch(() => {
      alert('Failed to copy to clipboard');
    });
  };

  if (loading) {
    return (
      <div className="page-container">
        <div className="loading">Loading SSH keys...</div>
      </div>
    );
  }

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <button className="btn btn-secondary btn-sm" onClick={() => navigate('/domains')}>
            ← Back to Domains
          </button>
          <h1>SSH Keys for {domain}</h1>
          {domainInfo && domainInfo.username && (
            <p className="text-muted">
              Domain user: <code>{domainInfo.username}</code> |
              Home: <code>/srv/{domain}/</code>
            </p>
          )}
        </div>
        <button className="btn btn-primary" onClick={() => setShowAddModal(true)}>
          + Add SSH Key
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {!domainInfo?.username && (
        <div className="alert alert-warning">
          <strong>No domain user configured.</strong>
          <p>This domain doesn't have a domain user. SSH key management is only available for domains with domain users.</p>
        </div>
      )}

      {keys.length === 0 ? (
        <div className="empty-state">
          <p>No SSH keys configured for this domain.</p>
          <button className="btn btn-primary" onClick={() => setShowAddModal(true)}>
            Add Your First SSH Key
          </button>
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
                  <td className="dns-content" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <code>{key.public_key.substring(0, 60)}...</code>
                    <button
                      className="btn btn-sm btn-secondary"
                      onClick={() => copyToClipboard(key.public_key)}
                      title="Copy full key"
                    >
                      📋
                    </button>
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

      <div className="card" style={{marginTop: '2rem'}}>
        <h3>How to Connect via SSH</h3>
        {domainInfo?.username ? (
          <div>
            <p>Once you've added your SSH public key, you can connect to the server:</p>
            <pre><code>ssh {domainInfo.username}@{domainInfo.server_ip || 'YOUR_SERVER_IP'}</code></pre>
            <p className="text-muted">
              You'll have access to <code>/srv/{domain}/</code> where you can deploy your applications.
            </p>
            <h4>Upload files with SCP or rsync:</h4>
            <pre><code>scp -r dist/* {domainInfo.username}@{domainInfo.server_ip || 'SERVER_IP'}:/srv/{domain}/www/src/</code></pre>
            <pre><code>rsync -avz --delete dist/ {domainInfo.username}@{domainInfo.server_ip || 'SERVER_IP'}:/srv/{domain}/www/src/</code></pre>
          </div>
        ) : (
          <p className="text-muted">Configure a domain user to enable SSH access.</p>
        )}
      </div>

      {showAddModal && (
        <SSHKeyModal
          domainName={domain}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
