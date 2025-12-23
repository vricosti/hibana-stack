import React, { useState, useEffect } from 'react';
import { useDomain } from '../context/DomainContext';
import { serviceAPI } from '../services/api';
import DeployModal from '../components/DeployModal';
import LogsModal from '../components/LogsModal';
import AddServiceModal from '../components/AddServiceModal';
import DeleteServiceModal from '../components/DeleteServiceModal';

function Services() {
  const { currentDomain } = useDomain();
  const [services, setServices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionLoading, setActionLoading] = useState({});

  // Modal states
  const [deployModal, setDeployModal] = useState({ open: false, service: null });
  const [logsModal, setLogsModal] = useState({ open: false, service: null, logs: '' });
  const [showAddModal, setShowAddModal] = useState(false);
  const [deleteModal, setDeleteModal] = useState({ open: false, service: null });

  useEffect(() => {
    if (currentDomain) {
      loadServices();
    }
  }, [currentDomain]);

  const loadServices = async () => {
    if (!currentDomain) return;

    try {
      setLoading(true);
      setError('');
      const data = await serviceAPI.list(currentDomain.name);
      setServices(data || []);
    } catch (err) {
      // Display detailed error message from API response
      const errorMessage = err.response?.data?.error || err.message || 'Failed to load services';
      setError(errorMessage);
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleAction = async (serviceName, action) => {
    const actionKey = `${serviceName}-${action}`;
    setActionLoading(prev => ({ ...prev, [actionKey]: true }));

    try {
      await serviceAPI[action](currentDomain.name, serviceName);
      // Refresh services after action
      await loadServices();
    } catch (err) {
      const errorMessage = err.response?.data?.error || err.message || `Failed to ${action} service`;
      setError(errorMessage);
    } finally {
      setActionLoading(prev => ({ ...prev, [actionKey]: false }));
    }
  };

  const handleViewLogs = async (service) => {
    setLogsModal({ open: true, service, logs: '', loading: true });

    try {
      const logs = await serviceAPI.getLogs(currentDomain.name, service.name);
      setLogsModal(prev => ({ ...prev, logs, loading: false }));
    } catch (err) {
      setLogsModal(prev => ({
        ...prev,
        logs: `Error loading logs: ${err.message}`,
        loading: false
      }));
    }
  };

  const handleRefreshLogs = async () => {
    if (!logsModal.service) return;

    setLogsModal(prev => ({ ...prev, loading: true }));
    try {
      const logs = await serviceAPI.getLogs(currentDomain.name, logsModal.service.name);
      setLogsModal(prev => ({ ...prev, logs, loading: false }));
    } catch (err) {
      setLogsModal(prev => ({
        ...prev,
        logs: `Error loading logs: ${err.message}`,
        loading: false
      }));
    }
  };

  const handleDeploy = (service) => {
    setDeployModal({ open: true, service });
  };

  const handleDeploySubmit = async (deployData) => {
    try {
      const result = await serviceAPI.deploy(
        currentDomain.name,
        deployModal.service.name,
        deployData
      );
      setDeployModal({ open: false, service: null });
      await loadServices();
      return result;
    } catch (err) {
      throw err;
    }
  };

  const getStatusBadge = (status) => {
    const statusStyles = {
      running: { background: '#28a745', color: 'white' },
      stopped: { background: '#dc3545', color: 'white' },
      not_deployed: { background: '#6c757d', color: 'white' },
      system: { background: '#17a2b8', color: 'white' }
    };

    const style = statusStyles[status] || statusStyles.not_deployed;

    return (
      <span
        style={{
          ...style,
          padding: '4px 12px',
          borderRadius: '12px',
          fontSize: '0.85em',
          fontWeight: '600',
          textTransform: 'capitalize'
        }}
      >
        {status.replace('_', ' ')}
      </span>
    );
  };

  const isActionLoading = (serviceName, action) => {
    return actionLoading[`${serviceName}-${action}`];
  };

  const getServiceUrl = (service) => {
    if (service.role === 'mailserver') return null;
    if (service.name === 'www') {
      return `https://www.${currentDomain.name}`;
    }
    return `https://${service.name}.${currentDomain.name}`;
  };

  const handleDeleteService = (service) => {
    if (!confirm(`Are you sure you want to delete the service "${service.name}"?\n\nThis will:\n• Stop and remove the container\n• Delete all files\n• Remove the DNS record`)) {
      return;
    }
    setDeleteModal({ open: true, service });
  };

  const handleDeleteModalClose = (reload) => {
    setDeleteModal({ open: false, service: null });
    if (reload) {
      loadServices();
    }
  };

  const performDelete = async () => {
    return serviceAPI.delete(currentDomain.name, deleteModal.service.name);
  };

  const handleAddModalClose = (reload) => {
    setShowAddModal(false);
    if (reload) {
      loadServices();
    }
  };

  if (!currentDomain) {
    return (
      <div className="empty-state">
        <p>Please select a domain to view services</p>
      </div>
    );
  }

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>Services</h2>
        <div style={{ display: 'flex', gap: '10px' }}>
          <button className="btn btn-secondary" onClick={loadServices}>
            Refresh
          </button>
          <button className="btn btn-primary" onClick={() => setShowAddModal(true)}>
            Add Service
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="table-container">
        <table className="dns-table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Role</th>
              <th>Container</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {services.map(service => (
              <tr key={service.name}>
                <td>
                  {getServiceUrl(service) ? (
                    <a
                      href={getServiceUrl(service)}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ color: '#007bff', textDecoration: 'none', fontWeight: 'bold' }}
                    >
                      {service.name}
                    </a>
                  ) : (
                    <strong>{service.name}</strong>
                  )}
                </td>
                <td>{service.role}</td>
                <td>
                  <code style={{ fontSize: '0.9em', color: '#666' }}>
                    {service.container_name || '-'}
                  </code>
                </td>
                <td>{getStatusBadge(service.status)}</td>
                <td>
                  <div className="table-actions">
                    {service.container_name && service.status !== 'system' && (
                      <>
                        {service.status === 'stopped' || service.status === 'not_deployed' ? (
                          <button
                            className="btn-icon btn-icon-success"
                            onClick={() => handleAction(service.name, 'start')}
                            disabled={isActionLoading(service.name, 'start')}
                            title="Start"
                          >
                            {isActionLoading(service.name, 'start') ? '...' : (
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                                <path d="M8 5v14l11-7z"/>
                              </svg>
                            )}
                          </button>
                        ) : (
                          <>
                            <button
                              className="btn-icon btn-icon-warning"
                              onClick={() => handleAction(service.name, 'restart')}
                              disabled={isActionLoading(service.name, 'restart')}
                              title="Restart"
                            >
                              {isActionLoading(service.name, 'restart') ? '...' : (
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                  <path d="M1 4v6h6M23 20v-6h-6"/>
                                  <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"/>
                                </svg>
                              )}
                            </button>
                            <button
                              className="btn-icon btn-icon-danger"
                              onClick={() => handleAction(service.name, 'stop')}
                              disabled={isActionLoading(service.name, 'stop')}
                              title="Stop"
                            >
                              {isActionLoading(service.name, 'stop') ? '...' : (
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                                  <rect x="6" y="6" width="12" height="12" rx="1"/>
                                </svg>
                              )}
                            </button>
                            <button
                              className="btn-icon"
                              onClick={() => handleViewLogs(service)}
                              title="Logs"
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                                <polyline points="14 2 14 8 20 8"/>
                                <line x1="16" y1="13" x2="8" y2="13"/>
                                <line x1="16" y1="17" x2="8" y2="17"/>
                              </svg>
                            </button>
                          </>
                        )}

                        {service.deployable && (
                          <button
                            className="btn btn-primary btn-sm"
                            onClick={() => handleDeploy(service)}
                          >
                            Deploy
                          </button>
                        )}

                        {service.is_custom && (
                          <button
                            className="btn-icon btn-icon-danger"
                            onClick={() => handleDeleteService(service)}
                            title="Delete service"
                          >
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                              <polyline points="3 6 5 6 21 6"/>
                              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                            </svg>
                          </button>
                        )}
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {services.length === 0 && (
        <div className="empty-state">
          <p>No services configured for this domain</p>
        </div>
      )}

      {/* Deploy Modal */}
      {deployModal.open && (
        <DeployModal
          service={deployModal.service}
          onClose={() => setDeployModal({ open: false, service: null })}
          onDeploy={handleDeploySubmit}
        />
      )}

      {/* Logs Modal */}
      {logsModal.open && (
        <LogsModal
          service={logsModal.service}
          logs={logsModal.logs}
          loading={logsModal.loading}
          onClose={() => setLogsModal({ open: false, service: null, logs: '' })}
          onRefresh={handleRefreshLogs}
        />
      )}

      {/* Add Service Modal */}
      {showAddModal && (
        <AddServiceModal
          domainName={currentDomain.name}
          onClose={handleAddModalClose}
        />
      )}

      {/* Delete Service Modal */}
      {deleteModal.open && (
        <DeleteServiceModal
          service={deleteModal.service}
          domainName={currentDomain.name}
          onClose={handleDeleteModalClose}
          onDelete={performDelete}
        />
      )}
    </div>
  );
}

export default Services;
