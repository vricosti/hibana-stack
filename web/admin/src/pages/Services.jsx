import React, { useState, useEffect } from 'react';
import { useDomain } from '../context/DomainContext';
import { serviceAPI } from '../services/api';
import DeployModal from '../components/DeployModal';
import LogsModal from '../components/LogsModal';

function Services() {
  const { currentDomain } = useDomain();
  const [services, setServices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionLoading, setActionLoading] = useState({});

  // Modal states
  const [deployModal, setDeployModal] = useState({ open: false, service: null });
  const [logsModal, setLogsModal] = useState({ open: false, service: null, logs: '' });

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
      setError('Failed to load services');
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
      setError(`Failed to ${action} service: ${err.message}`);
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
        <button className="btn btn-secondary" onClick={loadServices}>
          Refresh
        </button>
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
                  <strong>{service.name}</strong>
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
                            className="btn btn-primary btn-sm"
                            onClick={() => handleAction(service.name, 'start')}
                            disabled={isActionLoading(service.name, 'start')}
                          >
                            {isActionLoading(service.name, 'start') ? '...' : 'Start'}
                          </button>
                        ) : (
                          <>
                            <button
                              className="btn btn-secondary btn-sm"
                              onClick={() => handleAction(service.name, 'stop')}
                              disabled={isActionLoading(service.name, 'stop')}
                            >
                              {isActionLoading(service.name, 'stop') ? '...' : 'Stop'}
                            </button>
                            <button
                              className="btn btn-secondary btn-sm"
                              onClick={() => handleAction(service.name, 'restart')}
                              disabled={isActionLoading(service.name, 'restart')}
                            >
                              {isActionLoading(service.name, 'restart') ? '...' : 'Restart'}
                            </button>
                          </>
                        )}

                        {service.status === 'running' && (
                          <button
                            className="btn btn-secondary btn-sm"
                            onClick={() => handleViewLogs(service)}
                          >
                            Logs
                          </button>
                        )}

                        {service.deployable && (
                          <button
                            className="btn btn-primary btn-sm"
                            onClick={() => handleDeploy(service)}
                          >
                            Deploy
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
    </div>
  );
}

export default Services;
