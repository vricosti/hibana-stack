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
      window.location.href = '/login';
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
  remove: (domainName, fingerprint) => api.delete(`/domains/${domainName}/ssh-keys/${encodeURIComponent(fingerprint)}`),
  generate: () => api.post('/ssh-keys/generate').then(res => res.data.data)
};

export default api;
