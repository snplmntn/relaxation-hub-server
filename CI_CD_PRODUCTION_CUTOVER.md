# CI/CD Production Cutover

- [ ] Change CI/CD branch triggers from `dev` to `production`.
- [ ] Add and enable production deployment jobs.
- [ ] Configure production secrets and environment protections in GitHub.
- [ ] Add server deployment health checks and rollback steps.
- [ ] Add stronger hardening checks (for example `go test -race ./...`).

## Local Pre-Push Hook

Enable versioned git hooks for this repo:

```bash
git config core.hooksPath scripts/git-hooks
```
