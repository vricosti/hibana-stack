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
      setError('Failed to load emails');
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
      alert('Failed to delete email account');
    }
  };

  const handleDeleteAlias = async (alias) => {
    if (!window.confirm(`Delete redirect ${alias.source_address}?`)) return;

    try {
      await aliasAPI.delete(alias.id);
      loadData();
    } catch (err) {
      alert('Failed to delete redirect');
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
                      className="btn btn-secondary btn-sm"
                      onClick={() => item.type === 'account' ? handleEditEmail(item) : handleEditAlias(item)}
                    >
                      Edit
                    </button>
                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => item.type === 'account' ? handleDeleteEmail(item) : handleDeleteAlias(item)}
                    >
                      Delete
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
