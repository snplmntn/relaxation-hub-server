# Environment Variables Setup Guide

## Quick Start

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

Then update the values in `.env` with your actual configuration.

---

## Required Variables (Must Have)

### DATABASE_URL

**Purpose:** PostgreSQL database connection string  
**Format:** `postgresql://user:password@host:port/database`  
**Example:** `postgresql://postgres:password@localhost:5432/relaxation_hub`  
**Where to get:** Your PostgreSQL database credentials

```bash
DATABASE_URL=postgresql://postgres:mypassword@localhost:5432/relaxation_hub
```

### PORT

**Purpose:** Server listening port  
**Default:** `8080`  
**Format:** `8080`, `3000`, `5000`, etc.  
**Example:** `8080`

```bash
PORT=8080
```

### JWT_KEY

**Purpose:** Secret key for JWT token signing  
**Requirements:**

- Minimum 32 characters
- Use random, strong characters
- Keep it secret (don't commit to git)

**Generate a secure key:**

**On Linux/Mac:**

```bash
openssl rand -base64 32
```

**On Windows PowerShell:**

```powershell
[Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

**Example:**

```bash
JWT_KEY=KK7nVfJp9Lm5QwXzYaB2cDeFgHiJkLmNoPqRsT3uVwXyZ4=
```

---

## Optional Variables (OAuth)

### Google OAuth Setup

#### 1. Create Google OAuth Application

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project
3. Enable "Google+ API"
4. Go to "Credentials" → "Create Credentials" → "OAuth 2.0 Client ID"
5. Choose "Web application"
6. Add authorized redirect URI: `http://localhost:8080/oauth/callback`
7. Copy Client ID and Client Secret

#### 2. Set Environment Variables

```bash
GOOGLE_OAUTH_CLIENT_ID=123456789-abc123def456.apps.googleusercontent.com
GOOGLE_OAUTH_CLIENT_SECRET=GOCSPX-ABC123DEF456GHI789JKL012
GOOGLE_OAUTH_CALLBACK_URL=http://localhost:8080/oauth/callback
```

#### For Production

```bash
GOOGLE_OAUTH_CALLBACK_URL=https://yourdomain.com/oauth/callback
```

---

### Apple OAuth Setup

#### 1. Create Apple OAuth Application

1. Go to [Apple Developer Account](https://developer.apple.com/account/)
2. Sign in with Apple ID
3. Go to "Certificates, Identifiers & Profiles"
4. Create a new "Services ID"
5. Enable "Sign in with Apple"
6. Configure Return URLs: `http://localhost:8080/oauth/callback`
7. Create a new Key for the Services ID (Key type: Web Authentication Configuration)
8. Download the private key (.p8 file)

#### 2. Get Your IDs

**Team ID:**

- Top right corner of Apple Developer → Account
- Copy your Team ID

**Key ID:**

- Certificates, Identifiers & Profiles → Keys
- Copy the Key ID

**Private Key:**

- Download the .p8 file
- Read the contents

#### 3. Set Environment Variables

```bash
APPLE_OAUTH_CLIENT_ID=com.example.relaxationhub.signin
APPLE_OAUTH_CLIENT_SECRET=your-apple-client-secret
APPLE_OAUTH_CALLBACK_URL=http://localhost:8080/oauth/callback
APPLE_OAUTH_TEAM_ID=ABC123DEFG
APPLE_OAUTH_KEY_ID=XYZ123ABC
APPLE_OAUTH_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCx...
-----END PRIVATE KEY-----
```

**Note:** For multi-line private key in `.env`, use `\n`:

```bash
APPLE_OAUTH_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\nMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCx...\n-----END PRIVATE KEY-----
```

#### For Production

```bash
APPLE_OAUTH_CALLBACK_URL=https://yourdomain.com/oauth/callback
```

---

## Complete .env File Example

### Local Development

```bash
# Database (Local PostgreSQL)
DATABASE_URL=postgresql://postgres:password@localhost:5432/relaxation_hub

# Server
PORT=8080
JWT_KEY=KK7nVfJp9Lm5QwXzYaB2cDeFgHiJkLmNoPqRsT3uVwXyZ4=

# Google OAuth (Optional)
GOOGLE_OAUTH_CLIENT_ID=123456789-abc123def456.apps.googleusercontent.com
GOOGLE_OAUTH_CLIENT_SECRET=GOCSPX-ABC123DEF456GHI789JKL012
GOOGLE_OAUTH_CALLBACK_URL=http://localhost:8080/oauth/callback

# Apple OAuth (Optional)
APPLE_OAUTH_CLIENT_ID=com.example.relaxationhub.signin
APPLE_OAUTH_CLIENT_SECRET=your-secret
APPLE_OAUTH_CALLBACK_URL=http://localhost:8080/oauth/callback
APPLE_OAUTH_TEAM_ID=ABC123DEFG
APPLE_OAUTH_KEY_ID=XYZ123ABC
APPLE_OAUTH_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\nMIGfMA0...\n-----END PRIVATE KEY-----
```

### Staging/Production

```bash
# Database (Remote PostgreSQL)
DATABASE_URL=postgresql://user:password@staging-db.example.com:5432/relaxation_hub

# Server
PORT=8080
JWT_KEY=GenerateNewSecureKeyForProduction=

# Google OAuth
GOOGLE_OAUTH_CLIENT_ID=prod-client-id.apps.googleusercontent.com
GOOGLE_OAUTH_CLIENT_SECRET=prod-client-secret
GOOGLE_OAUTH_CALLBACK_URL=https://yourdomain.com/oauth/callback

# Apple OAuth
APPLE_OAUTH_CLIENT_ID=com.yourdomain.relaxationhub.signin
APPLE_OAUTH_CLIENT_SECRET=prod-secret
APPLE_OAUTH_CALLBACK_URL=https://yourdomain.com/oauth/callback
APPLE_OAUTH_TEAM_ID=ABC123DEFG
APPLE_OAUTH_KEY_ID=XYZ123ABC
APPLE_OAUTH_PRIVATE_KEY=-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----
```

---

## How to Load Environment Variables

### Option 1: .env File (Recommended for Development)

1. Create `.env` file in project root
2. Add your variables to `.env`
3. The application automatically loads from `.env` using `godotenv`

```bash
# .env
DATABASE_URL=postgresql://postgres:password@localhost:5432/relaxation_hub
PORT=8080
JWT_KEY=KK7nVfJp9Lm5QwXzYaB2cDeFgHiJkLmNoPqRsT3uVwXyZ4=
```

### Option 2: Environment Variables (Recommended for Production)

**Linux/Mac:**

```bash
export DATABASE_URL=postgresql://...
export PORT=8080
export JWT_KEY=...
export GOOGLE_OAUTH_CLIENT_ID=...
# ... etc
./server
```

**Windows PowerShell:**

```powershell
$env:DATABASE_URL = "postgresql://..."
$env:PORT = "8080"
$env:JWT_KEY = "..."
$env:GOOGLE_OAUTH_CLIENT_ID = "..."
# ... etc
.\server.exe
```

**Docker:**

```dockerfile
ENV DATABASE_URL=postgresql://...
ENV PORT=8080
ENV JWT_KEY=...
```

**Docker Compose:**

```yaml
environment:
  DATABASE_URL: postgresql://...
  PORT: 8080
  JWT_KEY: ...
```

### Option 3: Docker/Kubernetes Secrets

```yaml
# deployment.yaml
env:
  - name: JWT_KEY
    valueFrom:
      secretKeyRef:
        name: app-secrets
        key: jwt-key
```

---

## Security Best Practices

### ✅ DO:

- ✅ Generate strong JWT keys (32+ characters)
- ✅ Use different JWT keys for dev/staging/production
- ✅ Keep `.env` in `.gitignore` (don't commit secrets)
- ✅ Use environment variables in production
- ✅ Rotate secrets regularly
- ✅ Use HTTPS callback URLs in production
- ✅ Keep private keys secure
- ✅ Use secrets management tools (AWS Secrets Manager, HashiCorp Vault, etc.)

### ❌ DON'T:

- ❌ Commit `.env` file to git
- ❌ Share secrets in chat/email
- ❌ Use weak/simple JWT keys
- ❌ Use same secrets across environments
- ❌ Log environment variables
- ❌ Use HTTP callback URLs in production
- ❌ Hardcode secrets in code
- ❌ Expose private keys in logs

### .gitignore

```bash
# Environment variables - NEVER commit
.env
.env.local
.env.*.local
.env.production

# Keep .env.example - this is safe to commit
# .env.example is committed
```

---

## Verification Checklist

### Before Running the Server

- [ ] `.env` file exists in project root
- [ ] `DATABASE_URL` is set and valid
- [ ] `PORT` is set to available port
- [ ] `JWT_KEY` is 32+ characters
- [ ] Google OAuth variables set (if using Google login) OR all empty
- [ ] Apple OAuth variables set (if using Apple login) OR all empty
- [ ] No secrets in version control
- [ ] `.env` in `.gitignore`

### Testing the Setup

```bash
# Test database connection
make test-db

# Run the server
make run

# Should see:
# ✓ OAuth providers initialized
# ✓ Database connected
# ✓ Server running on :8080
```

### Verify OAuth Configuration

```bash
# Test Google OAuth endpoint
curl http://localhost:8080/oauth/google

# Test Apple OAuth endpoint
curl http://localhost:8080/oauth/apple

# Should redirect to OAuth provider or show 404 if not configured
```

---

## Troubleshooting

### "DATABASE_URL environment variable is required"

**Solution:** Make sure `.env` file exists and contains `DATABASE_URL`

### "JWT_KEY environment variable is required"

**Solution:** Add `JWT_KEY` to `.env` with 32+ characters

### "Invalid callback URL"

**Solution:** Ensure callback URL matches exactly in:

1. `.env` file
2. OAuth provider settings
3. Production domain (https for production)

### OAuth provider not working

**Solution:**

1. Check credentials are correct
2. Verify callback URL matches provider settings
3. Check client ID and secret are valid
4. Ensure provider is enabled in your account

### Private key issues (Apple)

**Solution:**

1. Verify `.p8` file is valid
2. Check for correct line breaks (`\n`)
3. Include `-----BEGIN/END PRIVATE KEY-----`
4. No extra whitespace

---

## Next Steps

1. ✅ Create `.env` file with required variables
2. ✅ Set up Google OAuth credentials (optional)
3. ✅ Set up Apple OAuth credentials (optional)
4. ✅ Test database connection
5. ✅ Run `make run` to start the server
6. ✅ Test OAuth flow in browser or Postman

---

## Helpful Commands

### Generate JWT Key

**Windows PowerShell:**

```powershell
[Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

### Test Database Connection

```bash
make test-db
```

### Run Server

```bash
make run
```

### Build Binary

```bash
make build
```

### Test OAuth Endpoints

```bash
# Google OAuth
curl http://localhost:8080/oauth/google

# Apple OAuth
curl http://localhost:8080/oauth/apple
```

---

## Support Resources

- [Google Cloud Console](https://console.cloud.google.com/)
- [Apple Developer Account](https://developer.apple.com/account/)
- [PostgreSQL Connection Strings](https://www.postgresql.org/docs/current/libpq-connect.html)
- [JWT Introduction](https://jwt.io/)
- [Goth Library Documentation](https://github.com/markbates/goth)

---

**Last Updated:** December 9, 2025  
**Status:** Production Ready
