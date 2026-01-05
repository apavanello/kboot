# Changelog

All notable changes to this project will be documented in this file.

## [2.3.4] - 2026-01-05
### Fixed
- Fixed bug where delete confirmation was not persisting selection.

## [2.3.3] - 2026-01-05
### Added
- Delete confirmation dialogs for clusters, static credentials, and SSO profiles.
- Prevents accidental deletion of critical configurations.

## [2.3.2] - 2026-01-05
### Added
- **Duplicate (`c` key)**: Quickly clone existing clusters and profiles.
- Pre-fills add form with existing data + suffix (e.g., `-copy`).
- Supports clusters, static credentials, and SSO profiles.
- Updated help bindings to include `c` key.

### Fixed
- Fixed SSO profile editing to correctly preserve `sso_start_url` by parsing `[sso-session]` blocks.

## [2.3.1] - 2026-01-05
### Added
- **Edit (`e` key)**: Added edit functionality for static credentials and SSO profiles.
- Editing credentials securely clears sensitive fields (keys/secrets) forcing re-entry.
- Editing acts as a "delete old + create new" operation to keep config files clean.
- Updated help text in forms with navigation hints (Tab/Shift+Tab).

## [2.3.0] - 2026-01-05
### Added
- **Unified TUI Dashboard**: `kboot config` now launches a comprehensive management interface.
- **Main Menu**: Navigate between "Manage Clusters" and "Manage AWS Credentials".
- **Credential Management**:
    - List view for `~/.aws/credentials` (Static) and `~/.aws/config` (SSO).
    - Add (`a`) and Delete (`d`) support for AWS credentials directly from TUI.
    - Masking of sensitive data (Access Keys, Tokens) in list view.
- **Cluster Management**:
    - Full CRUD support for EKS clusters config.
    - Improved form layouts with persistent headers.
- **Simplified CLI**: Removed standalone `auth` and `cluster` commands in favor of the unified dashboard.

## [2.2.0] - 2025-12-14
### Added
- Initial TUI support using `charmbracelet/bubbletea` and `huh`.
- `kboot config` command introduced.
- Interactive forms for adding clusters and credentials.

## [1.7.0] - 2025-11-20
### Added
- Multi-session SSO support.
- Parallel kubeconfig generation.
- Dynamic discovery of SSO sessions from `kboot.yaml`.
