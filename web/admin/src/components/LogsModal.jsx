import React from 'react';

export default function LogsModal({ service, logs, loading, onClose, onRefresh }) {
  return (
    <div className="modal-overlay" onClick={() => onClose()}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '800px' }}>
        <div className="modal-header">
          <h3>Logs: {service?.name}</h3>
          <button className="modal-close" onClick={() => onClose()}>x</button>
        </div>

        <div className="modal-body" style={{ padding: 0 }}>
          {loading ? (
            <div className="loading" style={{ padding: '40px' }}>
              <div className="spinner"></div>
            </div>
          ) : (
            <pre style={{
              background: '#1a1a2e',
              color: '#e0e0e0',
              padding: '20px',
              margin: 0,
              fontSize: '0.85em',
              fontFamily: "'Courier New', monospace",
              height: '500px',
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word'
            }}>
              {logs || 'No logs available'}
            </pre>
          )}
        </div>

        <div className="modal-footer">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onRefresh}
            disabled={loading}
          >
            {loading ? 'Loading...' : 'Refresh'}
          </button>
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
