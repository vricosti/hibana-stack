import { createContext, useContext, useState, useEffect } from 'react';
import { domainAPI } from '../services/api';
import { useAuth } from './AuthContext';

const DomainContext = createContext();

export function DomainProvider({ children }) {
  const { isAuthenticated } = useAuth();
  const [domains, setDomains] = useState([]);
  const [currentDomain, setCurrentDomain] = useState(null);
  const [loading, setLoading] = useState(true);

  // Load domains when authenticated (but don't auto-select)
  useEffect(() => {
    if (isAuthenticated) {
      loadDomains();
    } else {
      // Reset state when logged out
      setDomains([]);
      setCurrentDomain(null);
      setLoading(false);
    }
  }, [isAuthenticated]);

  // Persist selected domain in localStorage
  useEffect(() => {
    if (currentDomain) {
      localStorage.setItem('hibana_current_domain', JSON.stringify(currentDomain));
    } else {
      localStorage.removeItem('hibana_current_domain');
    }
  }, [currentDomain]);

  const loadDomains = async () => {
    try {
      setLoading(true);
      const response = await domainAPI.getAll();
      const domainList = response.data.data || [];
      setDomains(domainList);
      // Don't auto-select domain - let user choose from Domains page
    } catch (error) {
      console.error('Failed to load domains:', error);
    } finally {
      setLoading(false);
    }
  };

  const selectDomain = (domain) => {
    setCurrentDomain(domain);
  };

  const clearDomain = () => {
    setCurrentDomain(null);
  };

  const refreshDomains = () => {
    loadDomains();
  };

  return (
    <DomainContext.Provider value={{
      domains,
      currentDomain,
      selectDomain,
      clearDomain,
      refreshDomains,
      loading
    }}>
      {children}
    </DomainContext.Provider>
  );
}

export function useDomain() {
  const context = useContext(DomainContext);
  if (!context) {
    throw new Error('useDomain must be used within a DomainProvider');
  }
  return context;
}
