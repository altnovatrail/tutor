package provider

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// ResponseHeaderTimeout is how long an adapter waits for the headers of an answer once its request
// is sent: the time both SDKs give their own client, which the adapters do not take, since it
// comes with the defaults they take from the environment (docs/sources.md, 1.17). It does not
// bound the answer's body, so a stream goes on for as long as it lasts.
const ResponseHeaderTimeout = 10 * time.Minute

// transport is the adapters' one transport, so that their clients share its connections: the
// settings of Go's default transport, with ResponseHeaderTimeout.
var transport = sync.OnceValue(func() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: ResponseHeaderTimeout,
	}
})

// HTTPClient is the client the adapters send their requests by. Its transport is the adapters'
// own, not http.DefaultTransport, so that what a process puts there — an instrumentation that
// writes spans and sends the trace context to the provider — does not reach a call. What it takes
// from the process — the proxy of HTTPS_PROXY and NO_PROXY, the certificates of SSL_CERT_FILE and
// SSL_CERT_DIR — is the standard library's and the deployment's, as for every client of the
// process, and not a provider's (R105).
func HTTPClient() *http.Client {
	return &http.Client{Transport: transport()}
}
