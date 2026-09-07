# Security History Note

## Scope

This note records credential-shaped values found while preparing the
Community Backend baseline. Secret values are intentionally omitted.

The audit inspected all revisions reachable from the repository's current
refs for the affected configuration paths. No Git history was rewritten.

## Historical findings

| Risk | Path | Credential names | Historical evidence |
|---|---|---|---|
| Critical | `rag_service/config/config.yaml` | `llm.api_key` | A non-placeholder value exists in history, including commit `21d20714c654`. |
| High | `.env` | `JWT_ACCESS_TOKEN_KEY`, `JWT_REFRESH_TOKEN_KEY`, `WECHAT_STATE_TOKEN_KEY` | Non-placeholder values exist in history, including commit `fc7841a40dec`. |
| High | `config/dev.yaml` | `jwt.access_token_key`, `jwt.refresh_token_key`, `wechat.state_token_key` | Credential-shaped literals exist in history, including commit `21d20714c654`. |
| High | `config/worker.yaml` | `jwt.access_token_key`, `jwt.refresh_token_key`, `wechat.state_token_key` | Credential-shaped literals exist in history, including commit `21d20714c654`. |
| High | `config/notification.yaml` | `jwt.access_token_key`, `jwt.refresh_token_key`, `wechat.state_token_key` | Credential-shaped literals exist in history, including commit `21d20714c654`. |
| Low | `config/test.yaml` | JWT signing keys, WeChat test key and state key | Test-shaped literals exist in history, including commit `21d20714c654`; they must remain clearly non-production. |
| Low | `docker-compose.yaml` | Local MySQL root password | A fixed local-development credential has existed since commit `1e1578dab63e`; it must never be reused outside local development. |

## Required user actions

Assume any non-placeholder credential committed to a shared or remote
repository may have been disclosed.

Rotate, at minimum:

1. The external LLM provider API key.
2. JWT access-token and refresh-token signing keys.
3. The WeChat state-signing key.
4. Any other environment credential that reused one of these values.

Rotation must happen in the relevant provider or deployment environment.
Changing repository files alone does not revoke an exposed credential.

## History cleanup recommendation

`git filter-repo` is recommended later if this repository has been shared,
published, forked, or mirrored. History rewriting must be coordinated because
it changes commit IDs and requires collaborators to replace or carefully
repair existing clones.

Do not rewrite history until:

- credentials have already been rotated;
- a backup has been made;
- all affected branches and tags are identified;
- collaborators and remote administrators approve the force update.

M00-S deliberately does not run `git filter-repo`, force-push, or otherwise
rewrite existing history.
