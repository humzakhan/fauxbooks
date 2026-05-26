# Security Policy

## Supported versions

fauxbooks (`fbx`) is pre-1.0 and ships from a single line of development. Security
fixes target the **latest published release**, or `main` if no release is newer.
Older snapshots are not patched — upgrade to the latest version before reporting.

## Reporting a vulnerability

Please report security issues **privately**. Do not open a public GitHub issue for
a vulnerability.

Email **hello@humza.io** with:

- the `fbx` version (`fbx --version`),
- the exact command and flags that trigger the issue,
- a description of the impact and, if possible, a minimal reproduction.

You can expect a best-effort acknowledgement within **7 days**. We will work with
you on a fix and coordinate disclosure once a patch is available.

## Scope

`fbx` is a deterministic, offline generator of synthetic accounting data. It does
not read wall-clock time in the generation path and makes no network calls during
generation. Reports that are most useful concern, for example: path-traversal or
unexpected writes outside the output directory, resource-exhaustion from crafted
inputs, or any way to make a run produce non-deterministic output for the same
seed, bundle, period, and version.
