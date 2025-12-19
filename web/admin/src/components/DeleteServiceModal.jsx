import React, { useState, useEffect } from 'react';

export default function DeleteServiceModal({ service, domainName, onClose, onDelete }) {
  const [step, setStep] = useState(0);
  const [error, setError] = useState('');
  const [completed, setCompleted] = useState(false);

  const steps = [
    { label: 'Stopping container...', icon: '⏹' },
    { label: 'Removing container and images...', icon: '🗑' },
    { label: 'Deleting files...', icon: '📁' },
    { label: 'Removing DNS record...', icon: '🌐' },
    { label: 'Cleaning up database...', icon: '💾' },
  ];

  useEffect(() => {
    let interval;
    if (!error && !completed) {
      // Simulate progress through steps
      interval = setInterval(() => {
        setStep(prev => {
          if (prev < steps.length - 1) {
            return prev + 1;
          }
          return prev;
        });
      }, 800);
    }
    return () => clearInterval(interval);
  }, [error, completed]);

  useEffect(() => {
    const performDelete = async () => {
      try {
        await onDelete();
        setCompleted(true);
        setStep(steps.length);
        // Auto-close after success
        setTimeout(() => onClose(true), 1500);
      } catch (err) {
        setError(err.response?.data?.error || err.message || 'Failed to delete service');
      }
    };

    performDelete();
  }, []);

  return (
    <div className="modal-overlay">
      <div className="modal" style={{ maxWidth: '450px' }}>
        <div className="modal-header">
          <h3>Deleting {service.name}</h3>
        </div>
        <div className="modal-body">
          {error ? (
            <div className="alert alert-error" style={{ marginBottom: '20px' }}>
              {error}
            </div>
          ) : completed ? (
            <div className="alert" style={{
              background: '#d4edda',
              color: '#155724',
              border: '1px solid #c3e6cb',
              marginBottom: '20px',
              textAlign: 'center'
            }}>
              <span style={{ fontSize: '24px', marginRight: '10px' }}>✓</span>
              Service deleted successfully
            </div>
          ) : null}

          <div style={{ padding: '10px 0' }}>
            {steps.map((s, index) => (
              <div
                key={index}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  padding: '12px 0',
                  borderBottom: index < steps.length - 1 ? '1px solid #eee' : 'none',
                  opacity: index <= step ? 1 : 0.4,
                  transition: 'opacity 0.3s ease'
                }}
              >
                <span style={{
                  width: '30px',
                  height: '30px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  marginRight: '12px',
                  fontSize: '16px'
                }}>
                  {error && index === step ? (
                    <span style={{ color: '#dc3545' }}>✗</span>
                  ) : index < step || completed ? (
                    <span style={{ color: '#28a745' }}>✓</span>
                  ) : index === step ? (
                    <div className="spinner" style={{
                      width: '20px',
                      height: '20px',
                      borderWidth: '2px'
                    }}></div>
                  ) : (
                    <span style={{ color: '#ccc' }}>{s.icon}</span>
                  )}
                </span>
                <span style={{
                  color: index <= step ? '#333' : '#999',
                  fontWeight: index === step && !completed ? '600' : '400'
                }}>
                  {s.label}
                </span>
              </div>
            ))}
          </div>
        </div>
        <div className="modal-footer">
          {error || completed ? (
            <button
              className="btn btn-primary"
              onClick={() => onClose(completed)}
            >
              {completed ? 'Done' : 'Close'}
            </button>
          ) : (
            <span style={{ color: '#666', fontStyle: 'italic' }}>
              Please wait...
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
