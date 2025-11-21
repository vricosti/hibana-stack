# Hibana Admin Interface

React-based administration interface for Hibana Stack.

## Features

- **Dashboard**: Overview with statistics
- **Domain Management**: Add, edit, delete domains with automatic user creation
- **Email Account Management**: Manage email accounts across all domains
- **DNS Record Management**: CRUD operations for DNS records
- **JWT Authentication**: Secure API access

## Development

### Prerequisites

- Node.js 18+ and npm

### Install Dependencies

```bash
cd web/admin
npm install
```

### Development Server

```bash
npm run dev
```

The app will be available at http://localhost:5173

The Vite dev server proxies API requests to http://localhost:3000

### Build for Production

```bash
npm run build
```

The built files will be in the `dist/` directory.

## Project Structure

```
src/
├── components/       # Reusable React components
│   ├── DomainModal.jsx
│   ├── EmailModal.jsx
│   └── DNSModal.jsx
├── context/          # React contexts
│   └── AuthContext.jsx
├── pages/            # Page components
│   ├── Login.jsx
│   ├── Dashboard.jsx
│   ├── Domains.jsx
│   ├── Emails.jsx
│   └── DNS.jsx
├── services/         # API client
│   └── api.js
├── App.jsx           # Main app component
├── App.css           # Global styles
└── main.jsx          # Entry point
```

## API Integration

The app communicates with the Hibana API at `/api/v1`:

- **Auth**: POST `/api/v1/auth/login`
- **Domains**: GET/POST/PUT/DELETE `/api/v1/domains`
- **Emails**: GET/POST/PUT/DELETE `/api/v1/emails`
- **DNS**: GET/POST/PUT/DELETE `/api/v1/dns`
- **Stats**: GET `/api/v1/stats`

## Authentication

Login uses email account credentials from the database. The first email account created (typically `admin`) can be used to access the admin interface.

JWT tokens are stored in localStorage and included in all API requests via the Authorization header.

## Deployment

### With API Server

The API server can serve the built React app:

```bash
# Build the React app
cd web/admin
npm run build

# Start the API server with static file serving
cd ../..
./bin/hibana-api --port 3000 --static ./web/admin/dist
```

### Standalone with Nginx

Configure Nginx to serve the built files and proxy API requests:

```nginx
server {
    listen 80;
    server_name adm.example.com;

    root /path/to/hibana-stack/web/admin/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Technologies

- **React 18**: UI library
- **React Router 6**: Client-side routing
- **Axios**: HTTP client
- **Vite**: Build tool and dev server
