import React, { useState, useEffect } from 'react';
import { useDomain } from '../context/DomainContext';
import { emailAPI, aliasAPI } from '../services/api';
import EmailModal from '../components/EmailModal';
import AliasModal from '../components/AliasModal';

export default function Emails() {
  const { currentDomain } = useDomain();
  const [emails, setEmails] = useState([]);
  const [aliases, setAliases] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showEmailModal, setShowEmailModal] = useState(false);
  const [showAliasModal, setShowAliasModal] = useState(false);
  const [editingEmail, setEditingEmail] = useState(null);
  const [editingAlias, setEditingAlias] = useState(null);

  useEffect(() => {
    if (currentDomain) {
      loadData();
    }
  }, [currentDomain]);

  const loadData = async () => {
    if (!currentDomain) return;

    try {
      setLoading(true);
      const [emailsRes, aliasesRes] = await Promise.all([
        emailAPI.getByDomain(currentDomain.id),
        aliasAPI.getByDomain(currentDomain.id)
      ]);
      setEmails(emailsRes.data.data || []);
      setAliases(aliasesRes.data.data || []);
    } catch (err) {
      // Display detailed error message from API response
      const errorMessage = err.response?.data?.error || err.message || 'Failed to load emails';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const handleAddEmail = () => {
    setEditingEmail(null);
    setShowEmailModal(true);
  };

  const handleAddAlias = () => {
    setEditingAlias(null);
    setShowAliasModal(true);
  };

  const handleEditEmail = (email) => {
    setEditingEmail(email);
    setShowEmailModal(true);
  };

  const handleEditAlias = (alias) => {
    setEditingAlias(alias);
    setShowAliasModal(true);
  };

  const handleDeleteEmail = async (email) => {
    if (!window.confirm(`Delete email account ${email.username}?`)) return;

    try {
      await emailAPI.delete(email.id);
      loadData();
    } catch (err) {
      const errorMessage = err.response?.data?.error || err.message || 'Failed to delete email account';
      alert(errorMessage);
    }
  };

  const handleDeleteAlias = async (alias) => {
    if (!window.confirm(`Delete redirect ${alias.source_address}?`)) return;

    try {
      await aliasAPI.delete(alias.id);
      loadData();
    } catch (err) {
      const errorMessage = err.response?.data?.error || err.message || 'Failed to delete redirect';
      alert(errorMessage);
    }
  };

  const handleEmailModalClose = (refresh) => {
    setShowEmailModal(false);
    setEditingEmail(null);
    if (refresh) {
      loadData();
    }
  };

  const handleAliasModalClose = (refresh) => {
    setShowAliasModal(false);
    setEditingAlias(null);
    if (refresh) {
      loadData();
    }
  };

  if (!currentDomain) {
    return (
      <div className="empty-state">
        <p>Please select a domain to view email accounts</p>
      </div>
    );
  }

  if (loading) {
    return <div className="loading"><div className="spinner"></div></div>;
  }

  // Combine emails and aliases into a unified list
  const combinedList = [
    ...emails.map(e => ({ ...e, type: 'account', redirect: null })),
    ...aliases.map(a => ({ ...a, type: 'redirect', username: a.source_address, redirect: a.destination }))
  ];

  return (
    <div>
      <div className="page-header">
        <h2>Email Accounts</h2>
        <div className="header-buttons">
          <button className="btn btn-secondary" onClick={handleAddAlias}>
            Add Redirect
          </button>
          <button className="btn btn-primary" onClick={handleAddEmail}>
            Add Email Account
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {combinedList.length > 0 ? (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Username</th>
              <th>Redirect</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {combinedList.map((item) => (
              <tr key={`${item.type}-${item.id}`}>
                <td>{item.type === 'account' ? (item.full_name || '-') : '-'}</td>
                <td>{item.username}</td>
                <td>{item.redirect || '-'}</td>
                <td>
                  <div className="table-actions">
                    <button
                      className="btn-icon"
                      onClick={() => item.type === 'account' ? handleEditEmail(item) : handleEditAlias(item)}
                      title="Edit"
                    >
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                      </svg>
                    </button>
                    <button
                      className="btn-icon btn-icon-danger"
                      onClick={() => item.type === 'account' ? handleDeleteEmail(item) : handleDeleteAlias(item)}
                      title="Delete"
                    >
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <polyline points="3 6 5 6 21 6"/>
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                        <line x1="10" y1="11" x2="10" y2="17"/>
                        <line x1="14" y1="11" x2="14" y2="17"/>
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <div className="empty-state">
          <p>No email accounts for {currentDomain.name}. Add your first email account!</p>
        </div>
      )}

      {showEmailModal && (
        <EmailModal
          email={editingEmail}
          domainId={currentDomain.id}
          domainName={currentDomain.name}
          onClose={handleEmailModalClose}
        />
      )}

      {showAliasModal && (
        <AliasModal
          alias={editingAlias}
          domainId={currentDomain.id}
          domainName={currentDomain.name}
          onClose={handleAliasModalClose}
        />
      )}
    </div>
  );
}
