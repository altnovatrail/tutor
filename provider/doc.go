// Package provider is the port through which tutor reaches the AI providers. Each provider has
// an adapter that implements it, and a provider's SDK is imported by its adapter alone: that is
// how the SDKs reach api's build only through tutor.
package provider
