import React, { useState } from 'react';

export default function DeployModal({ service, onClose, onDeploy }) {
  const [source, setSource] = useState('git');
  const [gitUrl, setGitUrl] = useState('');
  const [gitBranch, setGitBranch] = useState('main');
  const [file, setFile] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    setResult(null);

    try {
      let deployData;

      if (source === 'git') {
        if (!gitUrl) {
          throw new Error('Git URL is required');
        }
        deployData = {
          source: 'git',
          git_url: gitUrl,
          git_branch: gitBranch || 'main'
        };
      } else {
        if (!file) {
          throw new Error('Please select a file to upload');
        }
        deployData = {
          source: 'upload',
          file: file
        };
      }

      const response = await onDeploy(deployData);
      setResult(response);
    } catch (err) {
      setError(err.response?.data?.error || err.message || 'Deployment failed');
    } finally {
      setLoading(false);
    }
  };

  const handleFileChange = (e) => {
    const selectedFile = e.target.files[0];
    if (selectedFile) {
      const allowedTypes = ['.zip', '.tar.gz', '.tgz', '.tar'];
      const fileName = selectedFile.name.toLowerCase();
      const isAllowed = allowedTypes.some(ext => fileName.endsWith(ext));

      if (!isAllowed) {
        setError('Please select a ZIP or TAR.GZ file');
        setFile(null);
        return;
      }

      setFile(selectedFile);
      setError('');
    }
  };

  // Show result after deployment
  if (result) {
    return (
      <div className="modal-overlay" onClick={() => onClose()}>
        <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '700px' }}>
          <div className="modal-header">
            <h3>Deployment {result.success ? 'Successful' : 'Failed'}</h3>
            <button className="modal-close" onClick={() => onClose()}>x</button>
          </div>

          <div className="modal-body">
            <div className={`alert ${result.success ? 'alert-success' : 'alert-error'}`}>
              {result.message}
            </div>

            {result.output && (
              <div className="form-group">
                <label>Deployment Output</label>
                <pre style={{
                  background: '#1a1a2e',
                  color: '#00ff00',
                  padding: '15px',
                  borderRadius: '5px',
                  fontSize: '0.85em',
                  maxHeight: '400px',
                  overflow: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word'
                }}>
                  {result.output}
                </pre>
              </div>
            )}
          </div>

          <div className="modal-footer">
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => onClose()}
            >
              Close
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="modal-overlay" onClick={() => onClose()}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Deploy {service?.name}</h3>
          <button className="modal-close" onClick={() => onClose()}>x</button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            {error && <div className="alert alert-error">{error}</div>}

            <div className="alert" style={{ background: '#fff3cd', color: '#856404', border: '1px solid #ffeeba' }}>
              <strong>Warning:</strong> The current content of the {service?.name} directory will be deleted before deployment. Configuration files (.ssh, .env, secrets) will be preserved.
            </div>

            <div className="form-group">
              <label>Source</label>
              <div style={{ display: 'flex', gap: '20px' }}>
                <label className="form-check">
                  <input
                    type="radio"
                    name="source"
                    value="git"
                    checked={source === 'git'}
                    onChange={(e) => setSource(e.target.value)}
                  />
                  Git Repository
                </label>
                <label className="form-check">
                  <input
                    type="radio"
                    name="source"
                    value="upload"
                    checked={source === 'upload'}
                    onChange={(e) => setSource(e.target.value)}
                  />
                  Upload Archive
                </label>
              </div>
            </div>

            {source === 'git' ? (
              <>
                <div className="form-group">
                  <label>Git URL *</label>
                  <input
                    type="text"
                    value={gitUrl}
                    onChange={(e) => setGitUrl(e.target.value)}
                    placeholder="git@github.com:user/repo.git"
                    required
                  />
                  <small>For private repos, use SSH format and configure SSH keys first</small>
                </div>

                <div className="form-group">
                  <label>Branch</label>
                  <input
                    type="text"
                    value={gitBranch}
                    onChange={(e) => setGitBranch(e.target.value)}
                    placeholder="main"
                  />
                  <small>Default: main</small>
                </div>
              </>
            ) : (
              <div className="form-group">
                <label>Archive File *</label>
                <input
                  type="file"
                  accept=".zip,.tar.gz,.tgz,.tar"
                  onChange={handleFileChange}
                  required
                />
                <small>Supported formats: ZIP, TAR.GZ, TGZ, TAR</small>
              </div>
            )}

            <div style={{ marginTop: '20px', padding: '15px', background: '#f5f5f5', borderRadius: '5px' }}>
              <strong>Deployment Process:</strong>
              <ol style={{ marginTop: '10px', paddingLeft: '20px', fontSize: '0.9em', color: '#666' }}>
                <li>Stop existing container</li>
                <li>Backup .ssh, .env, secrets</li>
                <li>Clear directory</li>
                <li>{source === 'git' ? 'Clone repository' : 'Extract archive'}</li>
                <li>Restore configuration files</li>
                <li>Run docker-compose up -d --build</li>
              </ol>
            </div>
          </div>

          <div className="modal-footer">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => onClose()}
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading}
            >
              {loading ? 'Deploying...' : 'Deploy'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
