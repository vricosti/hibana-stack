import React, { useState } from 'react';
import { sshKeyAPI } from '../services/api';

export default function SSHKeyModal({ domainName, onClose }) {
  const [keyType, setKeyType] = useState(null); // 'server' or 'external'
  const [formData, setFormData] = useState({
    public_key: '',
    private_key: '',
    label: '',
    host: '',
    hostname: '',
    user: ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [generating, setGenerating] = useState(false);
  const [generatedKeys, setGeneratedKeys] = useState(null);

  const handleSubmitServerKey = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await sshKeyAPI.add(domainName, {
        public_key: formData.public_key,
        label: formData.label
      });
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to add SSH key');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmitExternalKey = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await sshKeyAPI.addExternal(domainName, {
        private_key: formData.private_key,
        label: formData.label,
        host: formData.host,
        hostname: formData.hostname || formData.host,
        user: formData.user
      });
      onClose(true);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to add external key');
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

  const presetServices = [
    { name: 'GitHub', host: 'github.com', user: 'git' },
    { name: 'GitLab', host: 'gitlab.com', user: 'git' },
    { name: 'Bitbucket', host: 'bitbucket.org', user: 'git' },
    { name: 'Custom', host: '', user: '' }
  ];

  const handlePresetSelect = (preset) => {
    if (preset.name === 'Custom') {
      setFormData({ ...formData, host: '', hostname: '', user: '' });
    } else {
      setFormData({
        ...formData,
        host: preset.host,
        hostname: preset.host,
        user: preset.user,
        label: preset.name
      });
    }
  };

  const handleBack = () => {
    setKeyType(null);
    setFormData({
      public_key: '',
      private_key: '',
      label: '',
      host: '',
      hostname: '',
      user: ''
    });
    setError('');
    setGeneratedKeys(null);
  };

  // Step 1: Choose key type
  if (keyType === null) {
    return (
      <div className="modal-overlay" onClick={() => onClose(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header">
            <h3>Add SSH Key</h3>
            <button className="modal-close" onClick={() => onClose(false)}>×</button>
          </div>
          <div className="modal-body">
            <p style={{ marginBottom: '20px', color: '#666' }}>What type of SSH key do you want to add?</p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
              <button
                className="btn btn-secondary"
                onClick={() => setKeyType('server')}
                style={{ padding: '20px', textAlign: 'left' }}
              >
                <strong style={{ display: 'block', marginBottom: '5px' }}>Server Access Key</strong>
                <small style={{ color: '#666' }}>
                  Add a public key to connect to this server via SSH
                </small>
              </button>

              <button
                className="btn btn-secondary"
                onClick={() => setKeyType('external')}
                style={{ padding: '20px', textAlign: 'left' }}
              >
                <strong style={{ display: 'block', marginBottom: '5px' }}>External Service Key</strong>
                <small style={{ color: '#666' }}>
                  Add a private key for accessing external services (GitHub, GitLab, etc.)
                </small>
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Show generated keys for server access
  if (keyType === 'server' && generatedKeys) {
    return (
      <div className="modal-overlay" onClick={() => onClose(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header">
            <h3>SSH Key Generated</h3>
            <button className="modal-close" onClick={() => onClose(false)}>×</button>
          </div>
          <div className="modal-body">
            <div className="alert alert-success">
              <strong>SSH Key Pair Generated Successfully!</strong>
              <p style={{ marginTop: '10px', fontSize: '0.9em' }}>
                Download your private key now - you won't be able to retrieve it later.
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
      </div>
    );
  }

  // Server Access Key Form
  if (keyType === 'server') {
    return (
      <div className="modal-overlay" onClick={() => onClose(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header">
            <h3>Add Server Access Key</h3>
            <button className="modal-close" onClick={() => onClose(false)}>×</button>
          </div>
          <form onSubmit={handleSubmitServerKey}>
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
                  {generating ? 'Generating...' : 'Generate New SSH Key Pair'}
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
                onClick={handleBack}
                disabled={loading}
              >
                Back
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
        </div>
      </div>
    );
  }

  // External Service Key Form
  if (keyType === 'external') {
    return (
      <div className="modal-overlay" onClick={() => onClose(false)}>
        <div className="modal" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header">
            <h3>Add External Service Key</h3>
            <button className="modal-close" onClick={() => onClose(false)}>×</button>
          </div>
          <form onSubmit={handleSubmitExternalKey}>
            <div className="modal-body">
              {error && <div className="alert alert-error">{error}</div>}

              <div className="form-group">
                <label>Service</label>
                <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                  {presetServices.map((preset) => (
                    <button
                      key={preset.name}
                      type="button"
                      className={`btn btn-sm ${formData.host === preset.host && preset.name !== 'Custom' ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => handlePresetSelect(preset)}
                    >
                      {preset.name}
                    </button>
                  ))}
                </div>
              </div>

              <div className="form-group">
                <label>Label *</label>
                <input
                  type="text"
                  value={formData.label}
                  onChange={(e) => setFormData({ ...formData, label: e.target.value })}
                  placeholder="e.g., GitHub Deploy Key"
                  required
                />
              </div>

              <div className="form-group">
                <label>Host *</label>
                <input
                  type="text"
                  value={formData.host}
                  onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                  placeholder="e.g., github.com"
                  required
                />
                <small>The hostname to match in SSH config</small>
              </div>

              <div className="form-group">
                <label>User *</label>
                <input
                  type="text"
                  value={formData.user}
                  onChange={(e) => setFormData({ ...formData, user: e.target.value })}
                  placeholder="e.g., git"
                  required
                />
              </div>

              <div className="form-group">
                <label>Private Key *</label>
                <textarea
                  rows="10"
                  value={formData.private_key}
                  onChange={(e) => setFormData({ ...formData, private_key: e.target.value })}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
                  required
                  style={{ fontFamily: 'monospace', fontSize: '0.85em' }}
                />
                <small style={{ color: '#d32f2f' }}>⚠️ This key will be stored on the server in ~/.ssh/</small>
              </div>
            </div>

            <div className="modal-footer">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={handleBack}
                disabled={loading}
              >
                Back
              </button>
              <button
                type="submit"
                className="btn btn-primary"
                disabled={loading}
              >
                {loading ? 'Adding...' : 'Add External Key'}
              </button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  return null;
}
