# Plugin provider

The Plugin provider is a provider that allows DNS providers to integrate with ExternalDNS via an HTTP interface. The Plugin provider implements the Provider interface but instead of implementing code specific to a provider, it implements and HTTP client that sends request to an HTTP API. The idea behind this is that providers can be implemented in a separate program that exposes an HTTP API that the Plugin provider can interact with. The ideal setup for providers is to run as sidecars of the ExternalDNS container and listen on localhost only, but this is not strictly a requirement even if we do not recommend other setups.

## Architectural diagram

![Plugin provider](../img/plugin-provider.png)

## API guarantees

Providers implementing the HTTP API have to keep in sync with changes to the Go types `plan.Changes` and `endpoint.Endpoint`. We do not expect to make significant changes to those types given the maturity of the project, but we can't exclude that changes will need to happen. We commit to publishing changes to those in the release notes, to ensure that providers implementing the API can keep providers up to date quickly.

## Provider registry

To simplify the discovery of providers, we will accept pull requests that will add links to providers in the README file. This list will serve the only purpose of simplifying finding providers and will not constitute an official endorsment of any of the externally implemented providers unless otherwise specified.
