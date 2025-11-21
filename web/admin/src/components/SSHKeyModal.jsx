import React, { useState } from 'react';
import { sshKeyAPI } from '../services/api';

export default function SSHKeyModal({ domainName, onClose }) {
  const [formData, setFormData] = useState({
    public_key: '',
    label: ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [generating, setGenerating] = useState(false);
  const [generatedKeys, setGeneratedKeys] = useState(null);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await sshKeyAPI.add(domainName, formData);
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to add SSH key');
    } finally {
      setLoading(false);
    }
  };

  const handleGenerate = async () => {
    setGenerating(true);
    setError('');

    try {
      const keys = await sshKeyAPI.generate();
      setGeneratedKeys(keys);
      // Auto-fill the public key textarea
      setFormData({ ...formData, public_key: keys.public_key });
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to generate SSH key pair');
    } finally {
      setGenerating(false);
    }
  };

  const handleDownloadPrivateKey = () => {
    if (!generatedKeys) return;

    const blob = new Blob([generatedKeys.private_key], { type: 'text/plain' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${domainName}_id_ed25519`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  };

  const handleUseGenerated = () => {
    setGeneratedKeys(null);
  };

  return (
    <div className="modal-overlay" onClick={() => onClose(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>{generatedKeys ? 'SSH Key Generated' : 'Add SSH Key'}</h3>
          <button className="modal-close" onClick={() => onClose(false)}>×</button>
        </div>

        {generatedKeys ? (
          // Show generated keys
          <div>
            <div className="modal-body">
              <div className="alert alert-success">
                <strong>SSH Key Pair Generated Successfully!</strong>
                <p style={{ marginTop: '10px', fontSize: '0.9em' }}>
                  Your private key has been generated. Download it now - you won't be able to retrieve it later.
                </p>
              </div>

              <div className="form-group">
                <label>Public Key (will be added to authorized_keys)</label>
                <textarea
                  rows="3"
                  value={generatedKeys.public_key}
                  readOnly
                  style={{ fontFamily: 'monospace', fontSize: '0.85em', background: '#f5f5f5' }}
                />
              </div>

              <div className="form-group">
                <label>Private Key</label>
                <textarea
                  rows="10"
                  value={generatedKeys.private_key}
                  readOnly
                  style={{ fontFamily: 'monospace', fontSize: '0.85em', background: '#f5f5f5' }}
                />
                <small style={{ color: '#d32f2f' }}>⚠️ Keep this private key secure. Never share it with anyone.</small>
              </div>
            </div>

            <div className="modal-footer">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={handleDownloadPrivateKey}
              >
                Download Private Key
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleUseGenerated}
              >
                Continue to Add Key
              </button>
            </div>
          </div>
        ) : (
          // Show form
          <form onSubmit={handleSubmit}>
            <div className="modal-body">
              {error && <div className="alert alert-error">{error}</div>}

              <div style={{ marginBottom: '20px', textAlign: 'center' }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={handleGenerate}
                  disabled={generating}
                  style={{ width: '100%' }}
                >
                  {generating ? 'Generating...' : '🔑 Generate New SSH Key Pair'}
                </button>
                <small style={{ display: 'block', marginTop: '8px', color: '#666' }}>
                  Or paste your existing public key below
                </small>
              </div>

              <div className="form-group">
                <label>Label (Optional)</label>
                <input
                  type="text"
                  value={formData.label}
                  onChange={(e) => setFormData({ ...formData, label: e.target.value })}
                  placeholder="e.g., My Laptop, Work Computer"
                />
                <small>A friendly name to identify this key</small>
              </div>

              <div className="form-group">
                <label>SSH Public Key *</label>
                <textarea
                  rows="6"
                  value={formData.public_key}
                  onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
                  placeholder="ssh-rsa AAAAB3NzaC1... or ssh-ed25519 AAAAC3NzaC1..."
                  required
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
              <button
                type="submit"
                className="btn btn-primary"
                disabled={loading}
              >
                {loading ? 'Adding...' : 'Add SSH Key'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
