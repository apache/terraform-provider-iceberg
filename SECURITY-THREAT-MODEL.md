<!--
  Licensed to the Apache Software Foundation (ASF) under one
  or more contributor license agreements.  See the NOTICE file
  distributed with this work for additional information
  regarding copyright ownership.  The ASF licenses this file
  to you under the Apache License, Version 2.0 (the
  "License"); you may not use this file except in compliance
  with the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing,
  software distributed under the License is distributed on an
  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
  KIND, either express or implied.  See the License for the
  specific language governing permissions and limitations
  under the License.
-->

# Apache Iceberg Terraform Security Threat Model

This document describes the detailed security threat model for Apache
Iceberg Terraform. It is intended for maintainers and automated security
triage.

## Purpose

Apache Iceberg Terraform is a Terraform provider for configuring and managing
Iceberg-related control-plane resources. It is typically used by operators in
infrastructure-as-code workflows rather than by end users directly. Because of
that deployment model, many surprising or unsafe behaviors are deployment or
configuration issues rather than security vulnerabilities in the provider
itself.

This model is intended to answer:

- what Iceberg Terraform generally treats as a security vulnerability
- what Iceberg Terraform generally treats as correctness, hardening, or
  deployment work
- which boundaries are primarily owned by the provider versus the surrounding
  catalog, Terraform runtime, or operator
- which issue classes should be downgraded by default by scanners

## Scope

This model is scoped to the Apache Iceberg Terraform repository itself:

- provider configuration handling
- resource and data source behavior implemented in this repo
- provider-side transport, credential, and state handling

It is not a general threat model for every Terraform deployment that uses this
provider.

In particular, it does not attempt to define the complete security model for:

- the Terraform runtime, state backend, or execution environment
- storage-level authorization enforced outside the provider

## Security Goals

Iceberg Terraform should:

- avoid exposing secrets or delegated credentials to principals that were not
  already trusted with them
- avoid creating new unauthorized capabilities in provider-owned components
- avoid violating trust boundaries that the provider itself owns, such as
  leaking credential-bearing state across provider instances, workspaces, or
  resource boundaries it is expected to preserve

Iceberg Terraform does not aim to be the primary enforcement point for:

- operator authorization within Terraform itself
- storage-level authorization
- service-side credential scoping performed by an external catalog or control
  plane

## Roles

### Operator

The operator configures Terraform, the provider, the surrounding catalog, and
the storage integration. This role is trusted to choose endpoints,
credentials, and which resources the provider is allowed to manage.

### Catalog control plane

The catalog control plane resolves and manages Iceberg-related resources and
may return metadata, configuration, or delegated credentials to the provider.
Iceberg Terraform assumes this control plane is trusted and outside its
primary security boundary.

### Terraform provider instance

The provider instance consumes configuration, interacts with the catalog
control plane, and manages resources and data sources. Bugs in provider-side
routing, caching, state handling, or reuse may be security-relevant if they
leak credential-bearing state or cross boundaries the provider is expected to
preserve.

### Terraform operator workflow

Terraform plans, applies, state backends, and approval workflows are outside
the provider's primary security boundary unless the provider explicitly
documents a stronger guarantee.

## Trust Boundaries

### Boundary 1: operator-trusted configuration

The following are generally treated as trusted operator or deployment inputs:

- provider configuration
- endpoint configuration
- workspace and backend configuration
- credential configuration

If a report depends on the attacker controlling those values directly, it is
usually not a vulnerability in Iceberg Terraform itself.

### Boundary 2: catalog-supplied configuration and delegated access

The provider may accept metadata, configuration, endpoints, and delegated
access from the catalog control plane. By default, those are treated as
trusted control-plane inputs unless the provider explicitly documents a
stronger guarantee.

This means a malicious external control plane sending dangerous endpoints or
configuration is usually not an Iceberg Terraform vulnerability by itself. It
also means many delegated-access selection issues are often correctness or
specification issues rather than security boundary failures.

The major exception is secret exposure. If the provider surfaces credentials or
secrets to a new audience that was not already trusted with them, that is
security-relevant.

### Boundary 3: Terraform state and execution environment

Terraform state backends, plan output handling, and the execution environment
are not fully controlled by this provider. The provider should avoid leaking
secrets into state or logs unnecessarily, but it is not the root authority for
state backend security or execution-environment isolation.

## In-Scope Security Vulnerabilities

The following categories are generally security-relevant in Iceberg Terraform
when the report is credible and reproducible.

### 1. Secret or credential disclosure to a new audience

Examples include:

- provider credentials or delegated access leaked through state, logs, or
  user-visible output unexpectedly
- one provider instance's credentials or auth state leaking into another
  provider instance or workspace boundary the provider is expected to preserve

### 2. Provider-owned trust-boundary violations

Security issues exist when the provider itself is expected to separate
resources, provider instances, or principals and fails to do so.

Examples include:

- process-global auth or transport state crossing provider instances
- secret-bearing state from one resource or principal reused within another
  provider-owned boundary

## Usually Out of Scope or Non-Security by Default

These categories may still be real bugs worth fixing, but they are not usually
security vulnerabilities in Iceberg Terraform itself.

### 1. Correctness bugs

Examples include resource drift, plan/apply inconsistencies, or other logic
bugs that do not create a new trust-boundary violation.

### 2. Unsafe surrounding deployment or backend configuration

Reports that primarily depend on unsafe Terraform backend setup, unsafe state
handling outside the provider, or over-broad operator privileges are usually
deployment issues rather than provider security issues.

### 3. Malicious catalog or external control-plane scenarios

Reports that require a malicious catalog or other external control-plane
service are usually outside Iceberg Terraform's primary security boundary.

### 4. Equivalent-harm reports

If the actor already has a legitimate capability to achieve the same outcome
through provider configuration or Terraform control, the new path is usually
not a security issue.

## Scanner Calibration Rules

A scanner targeting Iceberg Terraform should treat a finding as
higher-confidence only if it plausibly shows one of the following:

- exposure of a secret or delegated credential to a new audience
- creation of a new unauthorized capability in a provider-owned component
- violation of a provider-owned trust boundary rather than a surrounding
  Terraform, catalog, service, or operator boundary

A finding should be downgraded or rejected by default if it instead depends
primarily on:

- unsafe backend or deployment configuration outside the provider
- a malicious catalog or external control-plane service
- a principal that already has equivalent power through legitimate Terraform
  configuration or operator privileges
