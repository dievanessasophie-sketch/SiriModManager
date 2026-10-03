# Code signing policy

**Status: preparation for a SignPath Foundation application.**
The project has not yet been accepted by SignPath Foundation and no Foundation
certificate or signed release is included in this source package.

## Project responsibilities

- Project maintainer, committer and reviewer: **Siri**, operator of siri-mods.de.
- Intended signing approver: **Siri**. Configure the actual authorized GitHub and
  SignPath accounts during onboarding; add other people only with their approval.
- Contributions from other people must be reviewed by the maintainer before merge.
- Everyone with source-control or signing access must enable MFA.

## Planned signing process

Only this project's own binaries built on GitHub-hosted Windows runners from the
public repository are eligible. SignPath must verify their GitHub origin. Signing
tokens belong in protected GitHub environment secrets, never in the source or EXE.

Every signing request requires approval by the authorized approver. A public
release is built from its protected `vX.Y.Z` tag. The workflow signs the manager
first, checks its signature and product metadata, embeds that exact file in the
installer, then requests the installer's signature. It checks the final signatures
and timestamp before producing distribution files and SHA-256 checksums.

Test and release policies and certificates are separate. Test output is marked
`TEST` and must not be advertised as a publicly trusted release. The workflow
does not automatically publish release assets.

The private signing key is held by the signing service. Certificate subject and
trusted publisher are determined by SignPath Foundation. Signing through the
Foundation does not promise that Windows will display `siri-mods.de` as publisher,
nor that SmartScreen will immediately stop warning about a new application.

## Acknowledgement after acceptance

Once the Foundation has actually accepted the project, update the status above
and include this acknowledgement on the repository home and download/release pages:

> Free code signing provided by [SignPath.io](https://signpath.io/), certificate
> by [SignPath Foundation](https://signpath.org/).

## Privacy policy

See [PRIVACY.md](PRIVACY.md) for network destinations, local data, optional periodic
requests and account revocation. The application uses the configured ModBase
service for its documented functions and does not contain analytics or advertising.

## References

- https://signpath.org/terms
- https://docs.signpath.io/trusted-build-systems/github
- https://docs.signpath.io/artifact-configuration/
- https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
