import React, { useState, useEffect } from 'react';
import { emailAPI, domainAPI } from '../services/api';
import EmailModal from '../components/EmailModal';

export default function Emails() {
  const [emails, setEmails] = useState([]);
  const [domains, setDomains] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingEmail, setEditingEmail] = useState(null);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const [emailsRes, domainsRes] = await Promise.all([
        emailAPI.getAll(),
        domainAPI.getAll()
      ]);
      setEmails(emailsRes.data.data || []);
      setDomains(domainsRes.data.data || []);
    } catch (err) {
      setError('Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingEmail(null);
    setShowModal(true);
  };

  const handleEdit = (email) => {
    setEditingEmail(email);
    setShowModal(true);
  };

  const handleDelete = async (email) => {
    if (!window.confirm(`Delete email account ${email.email}?`)) return;

    try {
      await emailAPI.delete(email.id);
      loadData();
    } catch (err) {
      alert('Failed to delete email account');
    }
  };

  const handleModalClose = (refresh) => {
    setShowModal(false);
    setEditingEmail(null);
    if (refresh) {
      loadData();
    }
  };

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>Email Accounts</h2>
        <button className="btn btn-primary" onClick={handleAdd}>
          Add Email Account
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {emails.length > 0 ? (
        <div className="data-table">
          {emails.map((email) => (
            <div key={email.id} className="data-item">
              <div className="data-item-info">
                <h4>{email.email}</h4>
                <p>{email.full_name}</p>
                <p>Domain: {email.domain_name}</p>
              </div>
              <div className="data-item-actions">
                <button
                  className="btn btn-secondary"
                  onClick={() => handleEdit(email)}
                >
                  Edit
                </button>
                <button
                  className="btn btn-danger"
                  onClick={() => handleDelete(email)}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="empty-state">
          <div className="empty-state-icon">📧</div>
          <p>No email accounts yet. Add your first email account!</p>
        </div>
      )}

      {showModal && (
        <EmailModal
          email={editingEmail}
          domains={domains}
          onClose={handleModalClose}
        />
      )}
    </div>
  );
}
