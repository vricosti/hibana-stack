import axios from 'axios';

const API_URL = '/api/v1';

// Create axios instance
const api = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json'
  }
});

// Add token to requests
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Handle 401 errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      // Don't redirect if already on login page to avoid infinite loop
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);

// Auth API
export const authAPI = {
  login: (username, password) =>
    api.post('/auth/login', { username, password }),

  logout: () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  }
};

// Domain API
export const domainAPI = {
  getAll: () => api.get('/domains'),
  list: () => api.get('/domains').then(res => res.data.data),
  getById: (id) => api.get(`/domains/${id}`),
  create: (data) => api.post('/domains', data),
  update: (id, data) => api.put(`/domains/${id}`, data),
  delete: (id) => api.delete(`/domains/${id}`),
  getStats: () => api.get('/stats')
};

// Email API
export const emailAPI = {
  getAll: () => api.get('/emails'),
  getByDomain: (domainId) => api.get(`/domains/${domainId}/emails`),
  getById: (id) => api.get(`/emails/${id}`),
  create: (domainId, data) => api.post(`/domains/${domainId}/emails`, data),
  update: (id, data) => api.put(`/emails/${id}`, data),
  delete: (id) => api.delete(`/emails/${id}`)
};

// Alias API (email redirections)
export const aliasAPI = {
  getByDomain: (domainId) => api.get(`/domains/${domainId}/aliases`),
  getById: (id) => api.get(`/aliases/${id}`),
  create: (domainId, data) => api.post(`/domains/${domainId}/aliases`, data),
  update: (id, data) => api.put(`/aliases/${id}`, data),
  delete: (id) => api.delete(`/aliases/${id}`)
};

// DNS API
export const dnsAPI = {
  getByDomain: (domainId) => api.get(`/domains/${domainId}/dns`),
  getById: (id) => api.get(`/dns/${id}`),
  create: (domainId, data) => api.post(`/domains/${domainId}/dns`, data),
  update: (id, data) => api.put(`/dns/${id}`, data),
  delete: (id) => api.delete(`/dns/${id}`)
};

// SSH Key API
export const sshKeyAPI = {
  list: (domainName) => api.get(`/domains/${domainName}/ssh-keys`).then(res => res.data.data),
  add: (domainName, data) => api.post(`/domains/${domainName}/ssh-keys`, data),
  addExternal: (domainName, data) => api.post(`/domains/${domainName}/ssh-keys/external`, data),
  remove: (domainName, fingerprint) => api.delete(`/domains/${domainName}/ssh-keys/${encodeURIComponent(fingerprint)}`),
  generate: () => api.post('/ssh-keys/generate').then(res => res.data.data)
};

// Config API
export const configAPI = {
  getDNSProvider: () => api.get('/config/dns-provider').then(res => res.data.data)
};

// DNS Provider API
export const dnsProviderAPI = {
  getAll: () => api.get('/dns-providers').then(res => res.data.data || []),
  getById: (id) => api.get(`/dns-providers/${id}`).then(res => res.data.data),
  create: (data) => api.post('/dns-providers', data),
  delete: (id) => api.delete(`/dns-providers/${id}`),
  testConnection: (data) => api.post('/dns-providers/test', data),
  getAvailableDomains: (id) => api.get(`/dns-providers/${id}/domains`).then(res => res.data.data || [])
};

// Service API (Docker management)
export const serviceAPI = {
  list: (domainName) => api.get(`/domains/${domainName}/services`).then(res => res.data.data),

  create: (domainName, data) =>
    api.post(`/domains/${domainName}/services`, data).then(res => res.data),

  delete: (domainName, serviceName) =>
    api.delete(`/domains/${domainName}/services/${serviceName}`).then(res => res.data),

  start: (domainName, serviceName) =>
    api.post(`/domains/${domainName}/services/${serviceName}/start`),

  stop: (domainName, serviceName) =>
    api.post(`/domains/${domainName}/services/${serviceName}/stop`),

  restart: (domainName, serviceName) =>
    api.post(`/domains/${domainName}/services/${serviceName}/restart`),

  getLogs: (domainName, serviceName, lines = 100) =>
    api.get(`/domains/${domainName}/services/${serviceName}/logs`, { params: { lines } })
      .then(res => res.data.data?.logs || ''),

  deploy: async (domainName, serviceName, data) => {
    if (data.source === 'upload' && data.file) {
      // Use FormData for file uploads
      const formData = new FormData();
      formData.append('source', 'upload');
      formData.append('file', data.file);

      const response = await api.post(
        `/domains/${domainName}/services/${serviceName}/deploy`,
        formData,
        { headers: { 'Content-Type': 'multipart/form-data' } }
      );
      return response.data.data;
    } else {
      // JSON for git deployment
      const response = await api.post(
        `/domains/${domainName}/services/${serviceName}/deploy`,
        data
      );
      return response.data.data;
    }
  }
};

export default api;
