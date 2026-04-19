# Report Dependency Matrix

This document is the operator-facing companion to the report dependency matrix implemented in `internal/handler/report_dependencies.go`.

## Dependencies

- `ledgerRepo`
- `bookingReferralRepo`
- `riderWalletService`
- `storageService`

## Endpoint Matrix

| Endpoint operation | Required dependencies |
| --- | --- |
| `GetLedgerSummary` | `ledgerRepo` |
| `GetLedgerTrend` | `ledgerRepo` |
| `GetReferralSummary` | `bookingReferralRepo` |
| `ListExpenses` | `ledgerRepo` |
| `CreateExpense` | `ledgerRepo` |
| `DeleteExpense` | `ledgerRepo` |
| `UploadExpenseReceipt` | `storageService` |
| `ListPayoutBalances` | `ledgerRepo` |
| `RecordSettlement` | `ledgerRepo` |
| `ListLedgerEntries` | `ledgerRepo` |
| `ListRiderPayoutRequests` | `riderWalletService` |
| `ResolveRiderPayoutRequest` | `riderWalletService` |

## Degraded Response Contract

Endpoints guarded by the matrix return `503 Service Unavailable` with:

```json
{
  "error": {
    "code": "dependency_unavailable",
    "message": "ledgerRepo is not configured",
    "dependency": "ledgerRepo",
    "retryable": true
  }
}
```

## Health Surface

`/health` and `/api/v1/health` expose:

- aggregate `status`
- aggregate `degraded`
- per-dependency availability under `dependencies`

## Endpoint-Specific Rule

- `ResolveRiderPayoutRequest` requires `ledgerRepo` only for the `approved` branch, because rejection only updates rider wallet state.
